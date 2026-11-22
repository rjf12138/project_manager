package core

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// 内置包：随 pm 二进制一起分发的基础第三方依赖。
//
// 设计要点：
//   - 仓库（git）里只保存源码，构建时由 build.sh/pkgtool 编译并打包，产物写入本目录；
//   - 二进制内同时内嵌「预编译 .pm」与「源码 tar.gz」：优先用预编译（零等待），
//     构建环境不匹配时回退到本机构建源码（约 10 秒）；
//   - 内置层只读，用户仓库优先，同名同版本以用户仓库为准。
//
//go:embed builtin
var builtinFS embed.FS

// BuiltinSpec 描述一个内置第三方包的源码与构建方式（third_party/<name>/pkg.json）
type BuiltinSpec struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description,omitempty"`
	License     string   `json:"license,omitempty"`
	LibType     LibType  `json:"lib_type,omitempty"`
	LinkLibs    []string `json:"link_libs,omitempty"`

	Build struct {
		System   string   `json:"system"`
		Src      string   `json:"src"`
		Args     []string `json:"args,omitempty"`
		CXXFlags string   `json:"cxxflags,omitempty"`
	} `json:"build"`

	Stage struct {
		Include []string `json:"include,omitempty"`
		Libs    []string `json:"libs,omitempty"`
	} `json:"stage"`

	Notes []string `json:"notes,omitempty"`
}

// BuiltEnv 记录内置包的构建环境，用于判断预编译产物能否直接使用
type BuiltEnv struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Compiler string `json:"compiler,omitempty"`
	CXXABI   int    `json:"cxx_abi,omitempty"`
	Flags    string `json:"flags,omitempty"`
	BuiltAt  string `json:"built_at,omitempty"`
}

// BuiltinEntry 内置清单中的一项
type BuiltinEntry struct {
	Spec   BuiltinSpec `json:"spec"`
	File   string      `json:"file"`             // packages/<name>-<ver>.pm
	SHA256 string      `json:"sha256"`           //
	Source string      `json:"source,omitempty"` // sources/<name>.tar.gz
	Built  BuiltEnv    `json:"built"`
}

// BuiltinIndex 内置清单（构建时由 pkgtool 生成）
type BuiltinIndex struct {
	Schema   int            `json:"schema"`
	Packages []BuiltinEntry `json:"packages"`
}

var (
	builtinOnce  sync.Once
	builtinCache *BuiltinIndex
)

// BuiltinIndexData 读取内嵌的内置包清单。
// 未执行 thirdparty 构建时返回空清单（不是错误），pm 仍可正常编译使用。
func BuiltinIndexData() *BuiltinIndex {
	builtinOnce.Do(func() {
		builtinCache = &BuiltinIndex{Schema: 1}
		data, err := builtinFS.ReadFile("builtin/index.json")
		if err != nil {
			return
		}
		var idx BuiltinIndex
		if err := json.Unmarshal(data, &idx); err != nil {
			return
		}
		builtinCache = &idx
	})
	return builtinCache
}

// BuiltinPackages 返回内置包清单（可能为空）
func BuiltinPackages() []BuiltinEntry {
	return BuiltinIndexData().Packages
}

// BuiltinEntryFor 按名字（可选版本）查找内置包清单项
func BuiltinEntryFor(name, version string) (BuiltinEntry, bool) {
	var best BuiltinEntry
	found := false
	for _, e := range BuiltinPackages() {
		if e.Spec.Name != name {
			continue
		}
		if version != "" {
			if e.Spec.Version == version {
				return e, true
			}
			continue
		}
		if !found || compareVersion(e.Spec.Version, best.Spec.Version) > 0 {
			best, found = e, true
		}
	}
	return best, found
}

// Compatible 判断预编译产物是否可用于当前环境。
// 只比 OS/Arch：这是硬性不匹配；编译器差异属于软性差异，交给源码兜底兜住。
func (e BuiltinEntry) Compatible() error {
	b := e.Built
	if b.OS == "" || b.Arch == "" {
		return nil // 未记录构建环境，不做判断
	}
	if b.OS != runtime.GOOS || b.Arch != runtime.GOARCH {
		return fmt.Errorf("built for %s-%s, current is %s-%s",
			b.OS, b.Arch, runtime.GOOS, runtime.GOARCH)
	}
	return nil
}

// MaterializeBuiltin 把内嵌的内置包释放到 ~/.project_manager/builtin/<key>/，
// 返回该目录（无内置包时返回空串）。已释放且校验通过时直接复用。
func MaterializeBuiltin() (string, error) {
	idx := BuiltinIndexData()
	if len(idx.Packages) == 0 {
		return "", nil
	}

	data, err := builtinFS.ReadFile("builtin/index.json")
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	dir := filepath.Join(ConfigRoot(), "builtin", hex.EncodeToString(sum[:6]))

	if builtinDirValid(dir, idx) {
		return dir, nil
	}

	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	// 释放 index.json 与 packages/、sources/
	err = fs.WalkDir(builtinFS, "builtin", func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			return nil
		}
		if path.Base(p) == "PLACEHOLDER.txt" {
			return nil
		}
		rel, rerr := filepath.Rel("builtin", filepath.FromSlash(p))
		if rerr != nil {
			return rerr
		}
		content, rerr := builtinFS.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		// 内置层在运行时是一个「平铺的只读仓库目录」：
		// packages/xxx.pm → <dir>/xxx.pm，这样 ListPackages / findPackageFile
		// 无需为内置层写特殊逻辑。sources/ 与 index.json 保持原样。
		rel = strings.TrimPrefix(rel, "packages"+string(os.PathSeparator))
		dst := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		return os.WriteFile(dst, content, 0644)
	})
	if err != nil {
		return "", err
	}

	if !builtinDirValid(dir, idx) {
		return "", fmt.Errorf("built-in packages failed checksum verification: %s", dir)
	}
	return dir, nil
}

// builtinDirValid 校验目录中的内置包是否齐全且摘要正确
func builtinDirValid(dir string, idx *BuiltinIndex) bool {
	for _, e := range idx.Packages {
		file := builtinMaterializedPath(dir, e.File)
		if e.SHA256 == "" {
			if _, err := os.Stat(file); err != nil {
				return false
			}
			continue
		}
		if err := VerifyPackage(file, e.SHA256); err != nil {
			return false
		}
	}
	return true
}

// builtinMaterializedPath 把清单里的相对路径映射成释放后的实际路径。
// 内置层是平铺的仓库目录，因此 packages/ 前缀会被去掉。
func builtinMaterializedPath(dir, rel string) string {
	p := filepath.FromSlash(rel)
	if strings.HasPrefix(p, "packages"+string(os.PathSeparator)) {
		p = strings.TrimPrefix(p, "packages"+string(os.PathSeparator))
	}
	return filepath.Join(dir, p)
}

// StageBuiltinPackage 按 spec.stage 规则把构建产物整理成 .pm 源目录。
// srcRoot 下应有 spec.Build.Src 指向的源码树；可选 LICENSE 会被一并放入包内。
func StageBuiltinPackage(srcRoot, stageDir string, spec BuiltinSpec) error {
	incDst := filepath.Join(stageDir, "include")
	for _, rel := range spec.Stage.Include {
		src := filepath.Join(srcRoot, filepath.FromSlash(rel))
		if !dirExists(src) {
			return fmt.Errorf("stage.include dir not found: %s", rel)
		}
		// 目录内容平铺 —— 这样安装后会落到 extern_inc/<包名>/ 下，
		// 例如 gtest 的 include/gtest/*.h → extern_inc/gtest/gtest.h，可用 #include <gtest/gtest.h>
		if err := copyDirContents(src, incDst); err != nil {
			return fmt.Errorf("failed to stage headers from %s: %w", rel, err)
		}
	}

	libRelease := filepath.Join(stageDir, "lib", "release")
	libDebug := filepath.Join(stageDir, "lib", "debug")
	for _, rel := range spec.Stage.Libs {
		src := filepath.Join(srcRoot, filepath.FromSlash(rel))
		if fi, err := os.Stat(src); err != nil || fi.IsDir() {
			return fmt.Errorf("stage.libs not found: %s", rel)
		}
		name := filepath.Base(src)
		// 同一份产物两种构建模式共用（静态库通常与模式无关）
		for _, dst := range []string{filepath.Join(libRelease, name), filepath.Join(libDebug, name)} {
			if err := copyFile(src, dst, 0644); err != nil {
				return fmt.Errorf("failed to stage lib %s: %w", name, err)
			}
		}
	}

	if lic := filepath.Join(srcRoot, "LICENSE"); fileExists(lic) {
		if err := copyFile(lic, filepath.Join(stageDir, "LICENSE"), 0644); err != nil {
			return err
		}
	}

	manifest := PackageManifest{
		Name:        spec.Name,
		Version:     spec.Version,
		Description: spec.Description,
		LibType:     spec.LibType,
		LinkLibs:    spec.LinkLibs,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(stageDir, "manifest.json"), data, 0644)
}

// BuildBuiltinFromSource 用内嵌源码在本机构建一个内置包，
// 并把结果打包进用户仓库。返回生成的 .pm 路径。
func BuildBuiltinFromSource(e BuiltinEntry, userRepo string) (string, error) {
	if e.Source == "" {
		return "", fmt.Errorf("built-in package %s has no bundled source", e.Spec.Name)
	}
	if e.Spec.Build.System != "cmake" {
		return "", fmt.Errorf("unsupported build system %q", e.Spec.Build.System)
	}

	archive := path.Join("builtin", filepath.ToSlash(e.Source))
	data, err := builtinFS.ReadFile(archive)
	if err != nil {
		return "", fmt.Errorf("bundled source not found (%s): %w", e.Source, err)
	}

	tmpDir, err := os.MkdirTemp("", "pm-builtin-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	tmpArchive := filepath.Join(tmpDir, "source.tar.gz")
	if err := os.WriteFile(tmpArchive, data, 0644); err != nil {
		return "", err
	}
	srcRoot := filepath.Join(tmpDir, "src-root")
	if err := os.MkdirAll(srcRoot, 0755); err != nil {
		return "", err
	}
	if err := extractArchive(tmpArchive, srcRoot); err != nil {
		return "", fmt.Errorf("failed to extract bundled source: %w", err)
	}

	buildRoot := filepath.Join(srcRoot, filepath.FromSlash(e.Spec.Build.Src))
	buildDir := filepath.Join(buildRoot, "build")

	cfgArgs := []string{"-S", buildRoot, "-B", buildDir, "-DCMAKE_BUILD_TYPE=Release"}
	cfgArgs = append(cfgArgs, e.Spec.Build.Args...)
	if f := e.Spec.Build.CXXFlags; f != "" {
		cfgArgs = append(cfgArgs, "-DCMAKE_CXX_FLAGS="+f)
	}
	if err := runCommand(buildRoot, "cmake", cfgArgs...); err != nil {
		return "", fmt.Errorf("cmake configure failed for %s: %w", e.Spec.Name, err)
	}
	if err := runCommand(buildRoot, "cmake", "--build", buildDir, "-j"); err != nil {
		return "", fmt.Errorf("cmake build failed for %s: %w", e.Spec.Name, err)
	}

	stageDir := filepath.Join(tmpDir, "stage")
	if err := os.MkdirAll(stageDir, 0755); err != nil {
		return "", err
	}
	if err := StageBuiltinPackage(srcRoot, stageDir, e.Spec); err != nil {
		return "", err
	}

	pm := NewPackageManagerWithRepo(userRepo)
	if err := os.MkdirAll(userRepo, 0755); err != nil {
		return "", err
	}
	out, err := pm.CreatePackageWithOptions(stageDir, CreateOptions{Force: true})
	if err != nil {
		return "", fmt.Errorf("failed to package %s: %w", e.Spec.Name, err)
	}
	return out, nil
}

// DetectCompiler 返回当前 C++ 编译器标识（用于记录内置包的构建环境）
func DetectCompiler() string {
	for _, bin := range []string{"c++", "g++", "clang++"} {
		out, err := exec.Command(bin, "--version").Output()
		if err != nil {
			continue
		}
		line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
		if line != "" {
			return line
		}
	}
	return "unknown"
}

// runCommand 在 dir 下执行命令，输出直通当前进程
func runCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	return cmd.Run()
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
