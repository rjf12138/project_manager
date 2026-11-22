package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// 没有执行 thirdparty 构建时内置清单为空，测试应优雅跳过
func requireBuiltin(t *testing.T, name string) BuiltinEntry {
	t.Helper()
	entry, ok := BuiltinEntryFor(name, "")
	if !ok {
		t.Skipf("内置包 %s 未构建（先运行 ./build.sh thirdparty）", name)
	}
	return entry
}

// 内置清单必须带有构建环境元数据
func TestBuiltinIndexHasBuildEnv(t *testing.T) {
	entry := requireBuiltin(t, "gtest")

	if entry.Spec.Version == "" {
		t.Errorf("内置包缺少版本号")
	}
	if entry.SHA256 == "" {
		t.Errorf("内置包缺少 sha256")
	}
	if entry.File == "" || entry.Source == "" {
		t.Errorf("内置包缺少 file/source: %+v", entry)
	}
	if entry.Built.OS == "" || entry.Built.Arch == "" {
		t.Errorf("内置包未记录构建环境: %+v", entry.Built)
	}
	if entry.Spec.Build.CXXFlags == "" {
		t.Errorf("内置包未记录构建旗标")
	}
	// 预编译产物绝不能绑死构建机 CPU
	if got := entry.Spec.Build.CXXFlags; len(got) > 0 && containsFlag(got, "-march=native") {
		t.Errorf("内置包构建旗标不能包含 -march=native: %s", got)
	}
}

func containsFlag(flags, want string) bool {
	for _, f := range filepath.SplitList(flags) {
		if f == want {
			return true
		}
	}
	return false
}

// 释放内置层：文件齐全、摘要正确，并且能被仓库扫描到
func TestMaterializeBuiltinAndList(t *testing.T) {
	requireBuiltin(t, "gtest")
	t.Setenv("HOME", t.TempDir())

	dir, err := MaterializeBuiltin()
	if err != nil {
		t.Fatalf("MaterializeBuiltin: %v", err)
	}
	if dir == "" {
		t.Fatal("内置层未释放")
	}

	entry, _ := BuiltinEntryFor("gtest", "")
	pmFile := builtinMaterializedPath(dir, entry.File)
	if _, err := os.Stat(pmFile); err != nil {
		t.Fatalf("内置包未释放到 %s: %v", pmFile, err)
	}
	if err := VerifyPackage(pmFile, entry.SHA256); err != nil {
		t.Errorf("内置包摘要校验失败: %v", err)
	}

	// 再次调用应复用同一目录（幂等）
	dir2, err := MaterializeBuiltin()
	if err != nil || dir2 != dir {
		t.Errorf("重复释放应复用目录: %q -> %q (err=%v)", dir, dir2, err)
	}

	pm := NewPackageManager()
	if err := pm.Init(); err != nil {
		t.Fatal(err)
	}
	pm.SetBuiltinDir(dir)

	infos, err := pm.ListPackages()
	if err != nil {
		t.Fatalf("ListPackages: %v", err)
	}
	found := false
	for _, info := range infos {
		if info.Manifest.Name == "gtest" {
			found = true
			if !pm.IsBuiltin(info.File) {
				t.Errorf("内置包应被识别为 builtin: %s", info.File)
			}
		}
	}
	if !found {
		t.Errorf("内置层中的 gtest 未被 ListPackages 发现: %+v", infos)
	}
}

// 用户仓库里的同名同版本包必须优先于内置层
func TestUserRepoShadowsBuiltin(t *testing.T) {
	entry := requireBuiltin(t, "gtest")
	t.Setenv("HOME", t.TempDir())

	dir, err := MaterializeBuiltin()
	if err != nil || dir == "" {
		t.Fatalf("MaterializeBuiltin: dir=%q err=%v", dir, err)
	}

	pm := NewPackageManager()
	if err := pm.Init(); err != nil {
		t.Fatal(err)
	}
	pm.SetBuiltinDir(dir)

	// 在用户仓库里放一个同名同版本的包
	src := t.TempDir()
	writeTestFile(t, filepath.Join(src, "manifest.json"),
		`{"name":"`+entry.Spec.Name+`","version":"`+entry.Spec.Version+`","description":"user override"}`)
	writeTestFile(t, filepath.Join(src, "include", "x.h"), "// x\n")
	userPkg, err := pm.CreatePackage(src, "")
	if err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}

	file, err := pm.findPackageFile(entry.Spec.Name, entry.Spec.Version)
	if err != nil {
		t.Fatalf("findPackageFile: %v", err)
	}
	if file != userPkg {
		t.Errorf("用户仓库应优先: got %s, want %s", file, userPkg)
	}
	if pm.IsBuiltin(file) {
		t.Errorf("用户仓库的包不应被识别为 builtin")
	}
}

// 平台不匹配时必须能被识别出来（源码兜底的前提）
func TestBuiltinCompatibilityCheck(t *testing.T) {
	entry := requireBuiltin(t, "gtest")

	if err := entry.Compatible(); err != nil {
		t.Errorf("当前构建的包应兼容本机: %v", err)
	}

	other := entry
	other.Built.OS = "plan9"
	other.Built.Arch = "mips"
	if err := other.Compatible(); err == nil {
		t.Errorf("跨平台的内置包应被判为不兼容")
	}

	unknown := entry
	unknown.Built.OS = ""
	unknown.Built.Arch = ""
	if err := unknown.Compatible(); err != nil {
		t.Errorf("未记录构建环境时不应报错: %v", err)
	}
}

// 源码兜底：用内嵌源码本机构建出可安装的 .pm
func TestBuiltinSourceFallbackBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过源码兜底构建")
	}
	if _, err := exec.LookPath("cmake"); err != nil {
		t.Skip("未安装 cmake")
	}
	entry := requireBuiltin(t, "gtest")

	t.Setenv("HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}

	out, err := BuildBuiltinFromSource(entry, repo)
	if err != nil {
		t.Fatalf("BuildBuiltinFromSource: %v", err)
	}
	if filepath.Dir(out) != repo {
		t.Errorf("产物应落在用户仓库 %s，实际 %s", repo, out)
	}
	if err := VerifyPackage(out, ""); err != nil {
		t.Errorf("兜底构建的包摘要校验失败: %v", err)
	}

	// 兜底产物应当可以被正常安装进项目
	proj := newTestProject(t, t.TempDir(), "FallbackApp")
	pm := NewPackageManagerWithRepo(repo)
	if _, err := pm.InstallToProject(proj, Dependency{Name: entry.Spec.Name, Version: entry.Spec.Version}); err != nil {
		t.Fatalf("安装兜底构建的包失败: %v", err)
	}
	incDir := filepath.Join(proj.DirInfo().ExternInc, entry.Spec.Name)
	if !dirExists(incDir) {
		t.Errorf("兜底构建的包头文件未安装: %s", incDir)
	}
}
