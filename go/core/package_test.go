package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"2.0", "1.9", 1},
		{"1.10.0", "1.9.0", 1},
		{"0.1.0", "0.1.0", 0},
		{"1.0", "1.0.0", 0},
		{"1.0.0-rc1", "1.0.0", -1},
		{"1.0.0", "1.0.0-rc1", 1},
		{"1.0.0-rc2", "1.0.0-rc10", -1},
		{"1.0.0-rc.1", "1.0.0-rc.2", -1},
		{"1.0.0-alpha", "1.0.0-beta", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha", 1},
		{"2.0.0-rc1", "1.9.0", 1},
		{"1.0.0+build1", "1.0.0+build2", 0},
	}
	for _, c := range cases {
		if got := compareVersion(c.a, c.b); got != c.want {
			t.Errorf("compareVersion(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// setupRepo 把 HOME 指向临时目录，隔离仓库与配置目录
func setupRepo(t *testing.T) *PackageManager {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	pm := NewPackageManager()
	if err := pm.Init(); err != nil {
		t.Fatalf("pm.Init: %v", err)
	}
	return pm
}

func writeTestFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCreatePackageAndInstall(t *testing.T) {
	pm := setupRepo(t)
	base := t.TempDir()

	src := filepath.Join(base, "mathlib")
	writeTestFile(t, filepath.Join(src, "manifest.json"),
		`{"name":"mathlib","version":"1.2.0","description":"math helpers","link_libs":["math"],"lib_type":"static"}`)
	writeTestFile(t, filepath.Join(src, "include", "mathlib.h"), "#pragma once\nint add(int, int);\n")
	writeTestFile(t, filepath.Join(src, "lib", "debug", "libmathlib.a"), "debug-lib")
	writeTestFile(t, filepath.Join(src, "lib", "release", "libmathlib.a"), "release-lib")

	archive, err := pm.CreatePackage(src, "")
	if err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("archive not created: %v", err)
	}
	if filepath.Base(archive) != "mathlib-1.2.0.pm" {
		t.Errorf("unexpected archive name: %s", filepath.Base(archive))
	}

	infos, err := pm.ListPackages()
	if err != nil {
		t.Fatalf("ListPackages: %v", err)
	}
	if len(infos) != 1 || infos[0].Manifest.Name != "mathlib" || infos[0].Manifest.Version != "1.2.0" {
		t.Fatalf("unexpected repository content: %+v", infos)
	}

	target := PackageTarget{
		Root:       base,
		ExternInc:  filepath.Join(base, "extern_inc"),
		LibDebug:   filepath.Join(base, "lib", "debug"),
		LibRelease: filepath.Join(base, "lib", "release"),
	}

	manifest, err := pm.Install(Dependency{Name: "mathlib", Version: "1.2.0"}, target)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if manifest.Name != "mathlib" || manifest.Version != "1.2.0" {
		t.Errorf("unexpected manifest: %+v", manifest)
	}
	if len(manifest.LinkLibs) != 1 || manifest.LinkLibs[0] != "math" {
		t.Errorf("link_libs not parsed: %+v", manifest.LinkLibs)
	}

	if _, err := os.Stat(filepath.Join(target.ExternInc, "mathlib", "mathlib.h")); err != nil {
		t.Errorf("header not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target.LibDebug, "libmathlib.a")); err != nil {
		t.Errorf("debug lib not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target.LibRelease, "libmathlib.a")); err != nil {
		t.Errorf("release lib not installed: %v", err)
	}
}

func TestInstallWithoutVersionPicksLatest(t *testing.T) {
	pm := setupRepo(t)
	base := t.TempDir()

	for _, v := range []string{"1.0.0", "1.2.0", "1.10.0"} {
		dir := filepath.Join(base, "libx", v)
		writeTestFile(t, filepath.Join(dir, "manifest.json"), `{"name":"libx","version":"`+v+`"}`)
		writeTestFile(t, filepath.Join(dir, "include", "x.h"), "// x\n")
		if _, err := pm.CreatePackage(dir, ""); err != nil {
			t.Fatalf("create %s: %v", v, err)
		}
	}

	file, err := pm.findPackageFile("libx", "")
	if err != nil {
		t.Fatalf("findPackageFile: %v", err)
	}
	if filepath.Base(file) != "libx-1.10.0.pm" {
		t.Errorf("expected libx-1.10.0.pm, got %s", filepath.Base(file))
	}
}

func TestInstallRecursiveDeps(t *testing.T) {
	pm := setupRepo(t)
	base := t.TempDir()

	writeTestFile(t, filepath.Join(base, "baselib", "manifest.json"),
		`{"name":"baselib","version":"1.0.0","link_libs":["base"]}`)
	writeTestFile(t, filepath.Join(base, "baselib", "include", "base.h"), "// base\n")
	if _, err := pm.CreatePackage(filepath.Join(base, "baselib"), ""); err != nil {
		t.Fatalf("create baselib: %v", err)
	}

	writeTestFile(t, filepath.Join(base, "toplib", "manifest.json"),
		`{"name":"toplib","version":"2.0.0","link_libs":["top"],"deps":[{"name":"baselib","version":"1.0.0"}]}`)
	writeTestFile(t, filepath.Join(base, "toplib", "include", "top.h"), "// top\n")
	if _, err := pm.CreatePackage(filepath.Join(base, "toplib"), ""); err != nil {
		t.Fatalf("create toplib: %v", err)
	}

	target := PackageTarget{
		Root:       base,
		ExternInc:  filepath.Join(base, "out", "extern_inc"),
		LibDebug:   filepath.Join(base, "out", "lib", "debug"),
		LibRelease: filepath.Join(base, "out", "lib", "release"),
	}

	manifests, err := pm.InstallWithDeps([]Dependency{{Name: "toplib"}}, target)
	if err != nil {
		t.Fatalf("InstallWithDeps: %v", err)
	}
	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(manifests))
	}

	for _, name := range []string{"toplib", "baselib"} {
		if _, err := os.Stat(filepath.Join(target.ExternInc, name)); err != nil {
			t.Errorf("dependency %s headers not installed: %v", name, err)
		}
	}
}

func TestSearchAndRemovePackage(t *testing.T) {
	pm := setupRepo(t)
	base := t.TempDir()

	writeTestFile(t, filepath.Join(base, "loglib", "manifest.json"),
		`{"name":"loglib","version":"0.3.0","description":"logging utility"}`)
	writeTestFile(t, filepath.Join(base, "loglib", "include", "log.h"), "// log\n")
	if _, err := pm.CreatePackage(filepath.Join(base, "loglib"), ""); err != nil {
		t.Fatal(err)
	}

	// 关键字匹配名称与描述
	if got, err := pm.SearchPackages("log"); err != nil || len(got) != 1 {
		t.Fatalf("SearchPackages(log) = %d hits, err=%v", len(got), err)
	}
	if got, err := pm.SearchPackages("utility"); err != nil || len(got) != 1 {
		t.Fatalf("SearchPackages(utility) = %d hits, err=%v", len(got), err)
	}
	if got, err := pm.SearchPackages("nope"); err != nil || len(got) != 0 {
		t.Fatalf("SearchPackages(nope) = %d hits, err=%v", len(got), err)
	}

	if err := pm.RemovePackage("loglib", ""); err != nil {
		t.Fatalf("RemovePackage: %v", err)
	}
	remaining, err := pm.ListPackages()
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected empty repository, got %d packages", len(remaining))
	}
}

// makePackage 在仓库中生成一个用于测试的包
func makePackage(t *testing.T, pm *PackageManager, base, name, version, manifestExtra string) string {
	t.Helper()
	src := filepath.Join(base, name+"-src", version)
	if manifestExtra == "" {
		manifestExtra = `,"link_libs":["` + name + `"]`
	}
	writeTestFile(t, filepath.Join(src, "manifest.json"),
		`{"name":"`+name+`","version":"`+version+`"`+manifestExtra+`}`)
	writeTestFile(t, filepath.Join(src, "include", name+".h"), "// "+name+"\n")
	writeTestFile(t, filepath.Join(src, "lib", "debug", "lib"+name+".a"), "debug-"+version)
	writeTestFile(t, filepath.Join(src, "lib", "release", "lib"+name+".a"), "release-"+version)

	file, err := pm.CreatePackage(src, "")
	if err != nil {
		t.Fatalf("CreatePackage(%s@%s): %v", name, version, err)
	}
	return file
}

// 版本号本身含 '-' 时也必须能找到并安装（回归：曾因从文件名反推版本而失效）
func TestFindPackageWithHyphenVersion(t *testing.T) {
	pm := setupRepo(t)
	base := t.TempDir()

	makePackage(t, pm, base, "rclib", "1.0.0-rc1", "")
	makePackage(t, pm, base, "rclib", "1.2.0-beta.1", "")

	// 精确版本
	file, err := pm.findPackageFile("rclib", "1.0.0-rc1")
	if err != nil {
		t.Fatalf("findPackageFile(rclib,1.0.0-rc1): %v", err)
	}
	if filepath.Base(file) != "rclib-1.0.0-rc1.pm" {
		t.Errorf("got %s, want rclib-1.0.0-rc1.pm", filepath.Base(file))
	}

	// 不指定版本 -> 取版本号最大的
	file, err = pm.findPackageFile("rclib", "")
	if err != nil {
		t.Fatalf("findPackageFile(rclib,''): %v", err)
	}
	if filepath.Base(file) != "rclib-1.2.0-beta.1.pm" {
		t.Errorf("got %s, want rclib-1.2.0-beta.1.pm", filepath.Base(file))
	}

	if m := pm.ManifestOf("rclib", "1.0.0-rc1"); m == nil || m.Version != "1.0.0-rc1" {
		t.Errorf("ManifestOf returned %+v", m)
	}
}

// 文件名与 manifest 里的包名/版本不一致时，也必须按 manifest 匹配
// （回归：旧实现用 LastIndex(base,"-") 从文件名反推，必然失败）
func TestFindPackageMatchesManifestNotFilename(t *testing.T) {
	pm := setupRepo(t)
	base := t.TempDir()

	src := filepath.Join(base, "src")
	writeTestFile(t, filepath.Join(src, "manifest.json"),
		`{"name":"realname","version":"2.0.0-rc2","link_libs":["realname"]}`)
	writeTestFile(t, filepath.Join(src, "include", "realname.h"), "// x\n")

	// 故意用一个与包名无关的文件名入库
	if _, err := pm.CreatePackageWithOptions(src, CreateOptions{
		OutFile: filepath.Join(pm.RepoDir(), "totally-different.pm"),
	}); err != nil {
		t.Fatalf("CreatePackageWithOptions: %v", err)
	}

	file, err := pm.findPackageFile("realname", "2.0.0-rc2")
	if err != nil {
		t.Fatalf("findPackageFile: %v", err)
	}
	if filepath.Base(file) != "totally-different.pm" {
		t.Errorf("got %s, want totally-different.pm", filepath.Base(file))
	}
}

// 同名同版本默认拒绝覆盖，--force 才允许
func TestCreatePackageRefusesOverwrite(t *testing.T) {
	pm := setupRepo(t)
	base := t.TempDir()

	makePackage(t, pm, base, "dup", "1.0.0", "")

	src := filepath.Join(base, "dup-src", "1.0.0")
	writeTestFile(t, filepath.Join(src, "include", "dup.h"), "// changed\n")

	_, err := pm.CreatePackage(src, "")
	if err == nil {
		t.Fatalf("expected an error when overwriting an existing name@version")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := pm.CreatePackageWithOptions(src, CreateOptions{Force: true}); err != nil {
		t.Fatalf("force overwrite failed: %v", err)
	}
}

// 打包时写入 sha256 边车，安装前校验；归档被篡改必须拒绝安装
func TestChecksumWrittenAndTamperRejected(t *testing.T) {
	pm := setupRepo(t)
	base := t.TempDir()

	archive := makePackage(t, pm, base, "secure", "1.0.0", "")

	if _, err := os.Stat(ChecksumSidecarPath(archive)); err != nil {
		t.Fatalf("checksum sidecar missing: %v", err)
	}
	if err := VerifyPackage(archive, ""); err != nil {
		t.Fatalf("VerifyPackage on fresh archive: %v", err)
	}

	// 校验失败：期望值与实际不符
	if err := VerifyPackage(archive, strings.Repeat("0", 64)); err == nil {
		t.Errorf("expected checksum mismatch")
	}

	// 篡改归档（追加字节）后安装必须失败
	f, err := os.OpenFile(archive, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("tampered")); err != nil {
		t.Fatal(err)
	}
	f.Close()

	target := PackageTarget{
		Root:       base,
		ExternInc:  filepath.Join(base, "out", "extern_inc"),
		LibDebug:   filepath.Join(base, "out", "lib", "debug"),
		LibRelease: filepath.Join(base, "out", "lib", "release"),
	}
	if _, err := pm.Install(Dependency{Name: "secure", Version: "1.0.0"}, target); err == nil {
		t.Errorf("expected install to reject a tampered archive")
	} else if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

// 没有边车文件的旧包仍然可以校验通过（向后兼容）
func TestVerifyPackageWithoutSidecar(t *testing.T) {
	pm := setupRepo(t)
	base := t.TempDir()

	archive := makePackage(t, pm, base, "legacy", "1.0.0", "")
	if err := os.Remove(ChecksumSidecarPath(archive)); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPackage(archive, ""); err != nil {
		t.Errorf("VerifyPackage without sidecar = %v, want nil", err)
	}
}

// 卸载依赖必须清掉复制进项目的头文件与库文件
func TestUninstallDependencyRemovesFiles(t *testing.T) {
	pm := setupRepo(t)
	base := t.TempDir()

	makePackage(t, pm, base, "greeter", "1.0.0", "")

	full, err := CreateProject("App", base)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pm.InstallToProject(proj, Dependency{Name: "greeter", Version: "1.0.0"}); err != nil {
		t.Fatalf("InstallToProject: %v", err)
	}

	headerDir := filepath.Join(proj.DirInfo().ExternInc, "greeter")
	libDebug := filepath.Join(proj.DirInfo().LibDebug, "libgreeter.a")
	libRelease := filepath.Join(proj.DirInfo().LibRelease, "libgreeter.a")
	for _, p := range []string{headerDir, libDebug, libRelease} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected %s to exist before uninstall: %v", p, err)
		}
	}

	removed, err := proj.UninstallDependency("greeter")
	if err != nil {
		t.Fatalf("UninstallDependency: %v", err)
	}
	for _, p := range []string{headerDir, libDebug, libRelease} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should be removed, stat err = %v", p, err)
		}
	}
	if len(removed) != 3 {
		t.Errorf("removed = %v, want 3 paths", removed)
	}

	if deps := proj.Config().Dependencies; len(deps) != 0 {
		t.Errorf("dependencies = %+v, want empty", deps)
	}
	if libs := proj.Config().LegacyDeps.Libs; len(libs) != 0 {
		t.Errorf("deps.libs = %v, want empty", libs)
	}
}

// 两个包共用同一个链接库时，卸载其中一个不能删掉库文件
func TestUninstallDependencyKeepsSharedLibrary(t *testing.T) {
	pm := setupRepo(t)
	base := t.TempDir()

	// alpha 与 beta 都声明 link_libs=["common"]
	writeTestFile(t, filepath.Join(base, "alpha", "manifest.json"),
		`{"name":"alpha","version":"1.0.0","link_libs":["common"]}`)
	writeTestFile(t, filepath.Join(base, "alpha", "include", "alpha.h"), "// a\n")
	writeTestFile(t, filepath.Join(base, "alpha", "lib", "debug", "libcommon.a"), "common")
	if _, err := pm.CreatePackage(filepath.Join(base, "alpha"), ""); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(base, "beta", "manifest.json"),
		`{"name":"beta","version":"1.0.0","link_libs":["common"]}`)
	writeTestFile(t, filepath.Join(base, "beta", "include", "beta.h"), "// b\n")
	writeTestFile(t, filepath.Join(base, "beta", "lib", "debug", "libcommon.a"), "common")
	if _, err := pm.CreatePackage(filepath.Join(base, "beta"), ""); err != nil {
		t.Fatal(err)
	}

	full, err := CreateProject("App2", base)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha", "beta"} {
		if _, err := pm.InstallToProject(proj, Dependency{Name: name, Version: "1.0.0"}); err != nil {
			t.Fatalf("install %s: %v", name, err)
		}
	}

	sharedLib := filepath.Join(proj.DirInfo().LibDebug, "libcommon.a")

	if _, err := proj.UninstallDependency("alpha"); err != nil {
		t.Fatalf("uninstall alpha: %v", err)
	}
	if _, err := os.Stat(sharedLib); err != nil {
		t.Errorf("shared lib must be kept while beta still needs it: %v", err)
	}
	if _, err := os.Stat(filepath.Join(proj.DirInfo().ExternInc, "alpha")); !os.IsNotExist(err) {
		t.Errorf("alpha headers should be removed")
	}

	if _, err := proj.UninstallDependency("beta"); err != nil {
		t.Fatalf("uninstall beta: %v", err)
	}
	if _, err := os.Stat(sharedLib); !os.IsNotExist(err) {
		t.Errorf("shared lib should be removed once no dependency needs it")
	}
}

// HasLibrary / LibraryFiles 只匹配指定库名对应的文件
func TestHasLibraryMatchesOnlyGivenName(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "libalpha.a"), "a")
	writeTestFile(t, filepath.Join(dir, "libbeta.so"), "b")
	writeTestFile(t, filepath.Join(dir, "libbeta.so.1"), "b")
	writeTestFile(t, filepath.Join(dir, "notes.txt"), "x")

	if !HasLibrary(dir, "alpha") {
		t.Errorf("HasLibrary(alpha) = false, want true")
	}
	if !HasLibrary(dir, "beta") {
		t.Errorf("HasLibrary(beta) = false, want true")
	}
	if HasLibrary(dir, "gamma") {
		t.Errorf("HasLibrary(gamma) = true, want false")
	}
	if got := len(LibraryFiles(dir, "beta")); got != 2 {
		t.Errorf("LibraryFiles(beta) = %d files, want 2", got)
	}
}

// ===================== 升级 / 自动登记 =====================

// newTestProject 创建并加载一个临时项目
func newTestProject(t *testing.T, base, name string) *Project {
	t.Helper()
	full, err := CreateProject(name, base)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	proj, err := LoadProject(full)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	return proj
}

// 升级依赖必须清掉旧版本残留：删除的头文件、改名后的旧库、失效的链接库名
func TestUpgradeCleansStaleArtifacts(t *testing.T) {
	pm := setupRepo(t)
	base := t.TempDir()

	// v1: include/{a.h,b.h}，库 libold.a，link_libs ["old"]
	v1 := filepath.Join(base, "mylib-v1")
	writeTestFile(t, filepath.Join(v1, "manifest.json"),
		`{"name":"mylib","version":"1.0.0","link_libs":["old"]}`)
	writeTestFile(t, filepath.Join(v1, "include", "a.h"), "a1")
	writeTestFile(t, filepath.Join(v1, "include", "b.h"), "b1")
	writeTestFile(t, filepath.Join(v1, "lib", "debug", "libold.a"), "old")
	if _, err := pm.CreatePackage(v1, ""); err != nil {
		t.Fatal(err)
	}

	proj := newTestProject(t, base, "App")
	if _, err := pm.InstallToProject(proj, Dependency{Name: "mylib", Version: "1.0.0"}); err != nil {
		t.Fatalf("install v1: %v", err)
	}

	// v2: 删掉 b.h，库改名 libnew.a，link_libs ["new"]
	v2 := filepath.Join(base, "mylib-v2")
	writeTestFile(t, filepath.Join(v2, "manifest.json"),
		`{"name":"mylib","version":"2.0.0","link_libs":["new"]}`)
	writeTestFile(t, filepath.Join(v2, "include", "a.h"), "a2")
	writeTestFile(t, filepath.Join(v2, "lib", "debug", "libnew.a"), "new")
	if _, err := pm.CreatePackage(v2, ""); err != nil {
		t.Fatal(err)
	}

	if _, err := pm.InstallToProject(proj, Dependency{Name: "mylib", Version: "2.0.0"}); err != nil {
		t.Fatalf("upgrade to v2: %v", err)
	}

	dirs := proj.DirInfo()

	// 旧头文件必须消失，同名的要更新为新内容
	if _, err := os.Stat(filepath.Join(dirs.ExternInc, "mylib", "b.h")); !os.IsNotExist(err) {
		t.Errorf("stale header b.h should be removed, stat err = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dirs.ExternInc, "mylib", "a.h"))
	if err != nil || string(data) != "a2" {
		t.Errorf("a.h = %q (err=%v), want updated content a2", data, err)
	}

	// 旧库文件必须消失
	if _, err := os.Stat(filepath.Join(dirs.LibDebug, "libold.a")); !os.IsNotExist(err) {
		t.Errorf("stale lib libold.a should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dirs.LibDebug, "libnew.a")); err != nil {
		t.Errorf("new lib libnew.a missing: %v", err)
	}

	// 链接库名不能累积
	if libs := proj.Config().LegacyDeps.Libs; len(libs) != 1 || libs[0] != "new" {
		t.Errorf("deps.libs = %v, want [new]", libs)
	}
	if deps := proj.Config().Dependencies; len(deps) != 1 || deps[0].Version != "2.0.0" {
		t.Errorf("dependencies = %+v, want single mylib@2.0.0", deps)
	}
}

// manifest 声明的额外 include_dirs / lib_dirs 必须自动登记进项目配置
func TestInstallRegistersPackageDirs(t *testing.T) {
	pm := setupRepo(t)
	base := t.TempDir()

	src := filepath.Join(base, "sdk-src")
	writeTestFile(t, filepath.Join(src, "manifest.json"),
		`{"name":"sdk","version":"1.0.0","link_libs":["sdk"],`+
			`"include_dirs":["include","third_party/dep"],`+
			`"lib_dirs":["lib","prebuilt"]}`)
	// 主头文件目录（平铺到 extern_inc/sdk/）
	writeTestFile(t, filepath.Join(src, "include", "sdk.h"), "// sdk\n")
	// 额外头文件目录（保持结构并需要登记）
	writeTestFile(t, filepath.Join(src, "third_party", "dep", "dep.h"), "// dep\n")
	// 约定库目录（落到 lib/debug，无需登记）
	writeTestFile(t, filepath.Join(src, "lib", "debug", "libsdk.a"), "sdk")
	// 非约定库目录（没有 debug/release，整目录搬到 lib/sdk/ 并登记）
	writeTestFile(t, filepath.Join(src, "prebuilt", "libextra.a"), "extra")

	if _, err := pm.CreatePackage(src, ""); err != nil {
		t.Fatal(err)
	}

	proj := newTestProject(t, base, "App")
	if _, err := pm.InstallToProject(proj, Dependency{Name: "sdk", Version: "1.0.0"}); err != nil {
		t.Fatalf("install: %v", err)
	}
	dirs := proj.DirInfo()
	cfg := proj.Config()

	// 落盘检查
	checks := []string{
		filepath.Join(dirs.ExternInc, "sdk", "sdk.h"),
		filepath.Join(dirs.ExternInc, "sdk", "third_party", "dep", "dep.h"),
		filepath.Join(dirs.LibDebug, "libsdk.a"),
		filepath.Join(dirs.Root, "lib", "sdk", "prebuilt", "libextra.a"),
	}
	for _, p := range checks {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist: %v", p, err)
		}
	}

	// 自动登记检查
	if got := cfg.LegacyDeps.IncDirs; len(got) != 1 || got[0] != "extern_inc/sdk/third_party/dep" {
		t.Errorf("deps.inc_dirs = %v, want [extern_inc/sdk/third_party/dep]", got)
	}
	if got := cfg.LegacyDeps.LibDirs; len(got) != 1 || got[0] != "lib/sdk/prebuilt" {
		t.Errorf("deps.lib_dirs = %v, want [lib/sdk/prebuilt]", got)
	}
	if got := cfg.LegacyDeps.Libs; len(got) != 1 || got[0] != "sdk" {
		t.Errorf("deps.libs = %v, want [sdk]", got)
	}
}

// 卸载要把自动登记的目录条目和目录本身一起清掉，手工配置的目录不能误删
func TestUninstallRemovesRegisteredDirs(t *testing.T) {
	pm := setupRepo(t)
	base := t.TempDir()

	src := filepath.Join(base, "sdk-src")
	writeTestFile(t, filepath.Join(src, "manifest.json"),
		`{"name":"sdk","version":"1.0.0","include_dirs":["include","extra"],"lib_dirs":["prebuilt"]}`)
	writeTestFile(t, filepath.Join(src, "include", "sdk.h"), "// sdk\n")
	writeTestFile(t, filepath.Join(src, "extra", "extra.h"), "// extra\n")
	writeTestFile(t, filepath.Join(src, "prebuilt", "libx.a"), "x")
	if _, err := pm.CreatePackage(src, ""); err != nil {
		t.Fatal(err)
	}

	proj := newTestProject(t, base, "App")
	if _, err := pm.InstallToProject(proj, Dependency{Name: "sdk"}); err != nil {
		t.Fatalf("install: %v", err)
	}
	dirs := proj.DirInfo()
	extraInc := filepath.Join(dirs.ExternInc, "sdk", "extra")
	extraLib := filepath.Join(dirs.Root, "lib", "sdk")

	if _, err := proj.UninstallDependency("sdk"); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	cfg := proj.Config()
	if len(cfg.LegacyDeps.IncDirs) != 0 {
		t.Errorf("deps.inc_dirs = %v, want empty", cfg.LegacyDeps.IncDirs)
	}
	if len(cfg.LegacyDeps.LibDirs) != 0 {
		t.Errorf("deps.lib_dirs = %v, want empty", cfg.LegacyDeps.LibDirs)
	}
	for _, p := range []string{extraInc, extraLib, filepath.Join(dirs.ExternInc, "sdk")} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should be removed, stat err = %v", p, err)
		}
	}
}

// 生成器必须使用 deps.inc_dirs / deps.lib_dirs（回归：这两个字段以前完全被忽略）
func TestGenerateCMakeUsesDependencyDirs(t *testing.T) {
	cfg := ProjectConfig{
		Name:         "App",
		Version:      "1.0.0",
		BuildMode:    BuildModeDebug,
		CXXStandard:  17,
		Language:     "CXX",
		CMakeMinimum: "3.10",
		OutputDir:    "output",
		LibDir:       "lib",
		ExternIncDir: "extern_inc",
		IncludeDirs:  []string{"inc"},
		SourceDirs:   []string{"src"},
		Targets:      []Target{{Name: "App", Type: "executable"}},
		LegacyDeps: ProjectDependencies{
			Libs:    []string{"sdk"},
			IncDirs: []string{"extern_inc/sdk/extra", "/opt/sdk/include"},
			LibDirs: []string{"lib/sdk/prebuilt", "/opt/sdk/lib"},
		},
	}

	out := GenerateCMakeLists(cfg, ProjectDirInfo{})

	want := []string{
		"include_directories(${CMAKE_SOURCE_DIR}/extern_inc/sdk/extra)",
		"include_directories(/opt/sdk/include)",
		"link_directories(${CMAKE_SOURCE_DIR}/lib/sdk/prebuilt)",
		"link_directories(/opt/sdk/lib)",
		"target_include_directories(App PRIVATE ${CMAKE_SOURCE_DIR}/extern_inc/sdk/extra)",
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("generated CMakeLists missing %q\n%s", w, out)
		}
	}
	// 绝对路径不能被拼成 ${CMAKE_SOURCE_DIR}/opt/...
	if strings.Contains(out, "${CMAKE_SOURCE_DIR}/opt/sdk") {
		t.Errorf("absolute path must not be prefixed with CMAKE_SOURCE_DIR:\n%s", out)
	}
}

// createTestPackage 在仓库中创建一个指定名称/版本的包，返回 .pm 路径。
func createTestPackage(t *testing.T, pm *PackageManager, name, version string) string {
	t.Helper()
	src := t.TempDir()
	writeTestFile(t, filepath.Join(src, "manifest.json"),
		fmt.Sprintf(`{"name":%q,"version":%q,"description":"%s %s","link_libs":[%q]}`,
			name, version, name, version, name))
	writeTestFile(t, filepath.Join(src, "include", name+".h"), "// "+name+"\n")
	writeTestFile(t, filepath.Join(src, "lib", "debug", "lib"+name+".a"), "debug-"+version)

	file, err := pm.CreatePackage(src, "")
	if err != nil {
		t.Fatalf("CreatePackage(%s@%s): %v", name, version, err)
	}
	return file
}

func decisionFor(result PruneResult, name, version string) *PruneDecision {
	for i := range result.Decisions {
		d := &result.Decisions[i]
		if d.Info.Manifest.Name == name && d.Info.Manifest.Version == version {
			return d
		}
	}
	return nil
}

func TestListPackageSummaries(t *testing.T) {
	pm := setupRepo(t)

	createTestPackage(t, pm, "alpha", "1.0.0")
	createTestPackage(t, pm, "alpha", "1.1.0")
	createTestPackage(t, pm, "alpha", "2.0.0")
	createTestPackage(t, pm, "beta", "1.0.0")

	summaries, err := pm.ListPackageSummaries()
	if err != nil {
		t.Fatalf("ListPackageSummaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("summaries = %d, want 2: %+v", len(summaries), summaries)
	}
	if summaries[0].Name != "alpha" || summaries[1].Name != "beta" {
		t.Fatalf("summaries order = %s, %s; want alpha, beta", summaries[0].Name, summaries[1].Name)
	}

	alpha := summaries[0]
	if alpha.Latest.Manifest.Version != "2.0.0" {
		t.Errorf("alpha latest = %s, want 2.0.0", alpha.Latest.Manifest.Version)
	}
	if len(alpha.Versions) != 3 {
		t.Fatalf("alpha versions = %d, want 3", len(alpha.Versions))
	}
	wantOrder := []string{"2.0.0", "1.1.0", "1.0.0"}
	for i, want := range wantOrder {
		if got := alpha.Versions[i].Manifest.Version; got != want {
			t.Errorf("alpha.Versions[%d] = %s, want %s", i, got, want)
		}
	}

	var wantSize int64
	for _, info := range alpha.Versions {
		wantSize += info.Size
	}
	if alpha.TotalSize != wantSize {
		t.Errorf("alpha.TotalSize = %d, want %d", alpha.TotalSize, wantSize)
	}

	// 显式指定版本时也必须返回同一个最新版
	latest, err := pm.LatestPackage("alpha")
	if err != nil {
		t.Fatalf("LatestPackage: %v", err)
	}
	if latest.Manifest.Version != "2.0.0" {
		t.Errorf("LatestPackage(alpha) = %s, want 2.0.0", latest.Manifest.Version)
	}

	if _, err := pm.LatestPackage("missing"); err == nil {
		t.Error("LatestPackage(missing) should fail")
	}
	versions, err := pm.PackageVersions("missing")
	if err != nil {
		t.Fatalf("PackageVersions(missing): %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("PackageVersions(missing) = %d, want 0", len(versions))
	}
}

func TestPrunePackages(t *testing.T) {
	pm := setupRepo(t)

	createTestPackage(t, pm, "alpha", "1.0.0")
	createTestPackage(t, pm, "alpha", "1.1.0")
	createTestPackage(t, pm, "alpha", "1.2.0")
	createTestPackage(t, pm, "beta", "1.0.0")
	createTestPackage(t, pm, "beta", "2.0.0")

	// alpha@1.0.0 被项目引用，应额外保留；其余按 keep=1 只保留最新。
	protected := map[string]map[string]bool{"alpha": {"1.0.0": true}}
	dry, err := pm.Prune(PruneOptions{Keep: 1, DryRun: true, Protected: protected})
	if err != nil {
		t.Fatalf("Prune(dry-run): %v", err)
	}

	want := map[string]string{
		"alpha@1.2.0": "latest",
		"alpha@1.0.0": "used-by-project",
		"alpha@1.1.0": "remove",
		"beta@2.0.0":  "latest",
		"beta@1.0.0":  "remove",
	}
	for key, reason := range want {
		parts := strings.SplitN(key, "@", 2)
		if len(parts) != 2 {
			t.Fatalf("bad test key %q", key)
		}
		name, version := parts[0], parts[1]
		d := decisionFor(dry, name, version)
		if d == nil {
			t.Fatalf("missing decision for %s", key)
		}
		if reason == "remove" {
			if d.Action != "remove" {
				t.Errorf("%s action = %s, want remove", key, d.Action)
			}
		} else if d.Action != "keep" || d.Reason != reason {
			t.Errorf("%s = %s/%s, want keep/%s", key, d.Action, d.Reason, reason)
		}
	}

	// dry-run 不能删文件
	if _, err := os.Stat(filepath.Join(pm.RepoDir(), "alpha-1.1.0.pm")); err != nil {
		t.Fatalf("dry-run removed alpha-1.1.0.pm: %v", err)
	}

	result, err := pm.Prune(PruneOptions{Keep: 1, Protected: protected})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(result.Removed) != 2 {
		t.Fatalf("removed = %d, want 2: %+v", len(result.Removed), result.Removed)
	}
	for _, removed := range []string{"alpha-1.1.0.pm", "beta-1.0.0.pm"} {
		if _, err := os.Stat(filepath.Join(pm.RepoDir(), removed)); !os.IsNotExist(err) {
			t.Errorf("%s should be removed, stat err = %v", removed, err)
		}
		if _, err := os.Stat(filepath.Join(pm.RepoDir(), removed+".sha256")); !os.IsNotExist(err) {
			t.Errorf("%s.sha256 should be removed, stat err = %v", removed, err)
		}
	}
	for _, kept := range []string{"alpha-1.0.0.pm", "alpha-1.2.0.pm", "beta-2.0.0.pm"} {
		if _, err := os.Stat(filepath.Join(pm.RepoDir(), kept)); err != nil {
			t.Errorf("%s should be kept: %v", kept, err)
		}
	}
	if result.Freed <= 0 {
		t.Errorf("Freed = %d, want > 0", result.Freed)
	}
}

func TestPruneKeepsBuiltinAndLatestStable(t *testing.T) {
	pm := setupRepo(t)

	// 内置层放 alpha@1.0.0，用户仓库放 alpha@2.0.0
	builtinDir := t.TempDir()
	pm.SetBuiltinDir(builtinDir)
	src := t.TempDir()
	writeTestFile(t, filepath.Join(src, "manifest.json"),
		`{"name":"alpha","version":"1.0.0","description":"builtin alpha"}`)
	writeTestFile(t, filepath.Join(src, "include", "alpha.h"), "// alpha\n")
	if _, err := pm.CreatePackageWithOptions(src, CreateOptions{
		OutFile: filepath.Join(builtinDir, "alpha-1.0.0.pm"),
	}); err != nil {
		t.Fatalf("create builtin package: %v", err)
	}

	createTestPackage(t, pm, "alpha", "2.0.0")

	result, err := pm.Prune(PruneOptions{Keep: 1, Force: true})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(result.Removed) != 0 {
		t.Fatalf("removed = %+v, want none (builtin must never be deleted)", result.Removed)
	}
	if _, err := os.Stat(filepath.Join(builtinDir, "alpha-1.0.0.pm")); err != nil {
		t.Errorf("builtin package removed: %v", err)
	}
	d := decisionFor(result, "alpha", "1.0.0")
	if d == nil || d.Action != "keep" || d.Reason != "builtin" {
		t.Errorf("builtin decision = %+v, want keep/builtin", d)
	}

	// 预发布版成为最新时，最高的正式版也要保留
	createTestPackage(t, pm, "gamma", "1.0.0")
	createTestPackage(t, pm, "gamma", "2.0.0-rc1")
	result, err = pm.Prune(PruneOptions{Keep: 1, Names: []string{"gamma"}})
	if err != nil {
		t.Fatalf("Prune(gamma): %v", err)
	}
	if len(result.Removed) != 0 {
		t.Fatalf("gamma removed = %+v, want none", result.Removed)
	}
	stable := decisionFor(result, "gamma", "1.0.0")
	if stable == nil || stable.Reason != "latest-stable" {
		t.Errorf("stable decision = %+v, want keep/latest-stable", stable)
	}
}

func TestPruneOrphanSidecar(t *testing.T) {
	pm := setupRepo(t)
	createTestPackage(t, pm, "alpha", "1.0.0")
	orphan := filepath.Join(pm.RepoDir(), "ghost.pm.sha256")
	writeTestFile(t, orphan, "deadbeef  ghost.pm\n")

	result, err := pm.Prune(PruneOptions{Keep: 1})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if len(result.OrphanSidecars) != 1 || result.OrphanSidecars[0] != orphan {
		t.Fatalf("OrphanSidecars = %+v, want [%s]", result.OrphanSidecars, orphan)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Errorf("orphan sidecar should be removed, stat err = %v", err)
	}
}
