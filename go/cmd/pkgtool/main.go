// Command pkgtool 把 third_party/ 下的第三方源码编译并打包成 .pm，
// 产出到 go/core/builtin/，供 pm 二进制内嵌。
//
// 它直接复用 core 的打包实现，因此不依赖已经编译好的 pm 二进制
// —— 解决了「用 pm 打包 pm 自己要内嵌的包」这个鸡生蛋问题。
//
// 用法（通常由 build.sh 调用）：
//
//	go run ./cmd/pkgtool -third-party ../third_party -out core/builtin
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/rjf12138/project_manager/core"
)

func main() {
	thirdPartyDir := flag.String("third-party", "../third_party", "第三方源码根目录")
	outDir := flag.String("out", "core/builtin", "内置包产物目录（相对 go/ 模块根）")
	only := flag.String("name", "", "只处理指定包（默认全部）")
	force := flag.Bool("force", false, "忽略缓存强制重建")
	keepBuild := flag.Bool("keep-build", false, "保留源码目录下的 build/")
	flag.Parse()

	if err := run(*thirdPartyDir, *outDir, *only, *force, *keepBuild); err != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] %v\n", err)
		os.Exit(1)
	}
}

func run(thirdPartyDir, outDir, only string, force, keepBuild bool) error {
	// 统一转绝对路径：cmake 的 -S/-B 是相对 cmd.Dir 解析的，混用相对路径会出错
	var err error
	if thirdPartyDir, err = filepath.Abs(thirdPartyDir); err != nil {
		return err
	}
	if outDir, err = filepath.Abs(outDir); err != nil {
		return err
	}

	pkgDirs, err := discover(thirdPartyDir, only)
	if err != nil {
		return err
	}
	if len(pkgDirs) == 0 {
		fmt.Println("[INFO] third_party/ 下没有包，跳过")
		return nil
	}

	packagesDir := filepath.Join(outDir, "packages")
	sourcesDir := filepath.Join(outDir, "sources")
	stampsDir := filepath.Join(outDir, ".stamps")
	for _, d := range []string{packagesDir, sourcesDir, stampsDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	// 增量：以已有清单为基础，本次处理的包做 upsert
	index := loadIndex(filepath.Join(outDir, "index.json"))
	index.Schema = 1

	for _, pkgDir := range pkgDirs {
		entry, err := processPackage(pkgDir, outDir, packagesDir, sourcesDir, stampsDir, force, keepBuild)
		if err != nil {
			return err
		}
		upsert(&index, entry)
		fmt.Printf("[ OK ] 内置包就绪: %s@%s (%s, sha256=%s)\n",
			entry.Spec.Name, entry.Spec.Version, entry.File, entry.SHA256[:12])
	}

	sort.Slice(index.Packages, func(i, j int) bool {
		return index.Packages[i].Spec.Name < index.Packages[j].Spec.Name
	})
	return writeIndex(filepath.Join(outDir, "index.json"), index)
}

// discover 找出 third_party/<name>/pkg.json 形式的包目录
func discover(root, only string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if only != "" && e.Name() != only {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if !fileExists(filepath.Join(dir, "pkg.json")) {
			continue
		}
		dirs = append(dirs, dir)
	}
	return dirs, nil
}

func processPackage(pkgDir, outDir, packagesDir, sourcesDir, stampsDir string, force, keepBuild bool) (core.BuiltinEntry, error) {
	spec, err := loadSpec(filepath.Join(pkgDir, "pkg.json"))
	if err != nil {
		return core.BuiltinEntry{}, fmt.Errorf("%s: %w", pkgDir, err)
	}
	if spec.Name == "" || spec.Version == "" {
		return core.BuiltinEntry{}, fmt.Errorf("%s: pkg.json 缺少 name/version", pkgDir)
	}

	pmFile := filepath.Join(packagesDir, fmt.Sprintf("%s-%s.pm", spec.Name, spec.Version))
	srcFile := filepath.Join(sourcesDir, spec.Name+".tar.gz")
	stampFile := filepath.Join(stampsDir, spec.Name)

	hash, err := treeHash(pkgDir)
	if err != nil {
		return core.BuiltinEntry{}, err
	}

	cached := !force && stampMatches(stampFile, hash) && fileExists(pmFile) && fileExists(srcFile)
	if cached {
		fmt.Printf("[INFO] %s@%s 未变化，跳过编译\n", spec.Name, spec.Version)
	} else {
		fmt.Printf("[INFO] 构建 %s@%s ...\n", spec.Name, spec.Version)
		if err := build(pkgDir, spec, keepBuild); err != nil {
			return core.BuiltinEntry{}, err
		}
		if err := stageAndPack(pkgDir, spec, packagesDir); err != nil {
			return core.BuiltinEntry{}, err
		}
		if err := makeSourceArchive(pkgDir, srcFile); err != nil {
			return core.BuiltinEntry{}, err
		}
		if err := os.WriteFile(stampFile, []byte(hash), 0644); err != nil {
			return core.BuiltinEntry{}, err
		}
	}

	sum, err := core.FileSHA256(pmFile)
	if err != nil {
		return core.BuiltinEntry{}, err
	}

	return core.BuiltinEntry{
		Spec:   spec,
		File:   relSlash(outDir, pmFile),
		SHA256: sum,
		Source: relSlash(outDir, srcFile),
		Built: core.BuiltEnv{
			OS:       runtime.GOOS,
			Arch:     runtime.GOARCH,
			Compiler: core.DetectCompiler(),
			CXXABI:   1, // GCC5+ 默认 _GLIBCXX_USE_CXX11_ABI=1
			Flags:    spec.Build.CXXFlags,
			BuiltAt:  time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

// build 按 spec 编译第三方源码
func build(pkgDir string, spec core.BuiltinSpec, keepBuild bool) error {
	if spec.Build.System != "cmake" {
		return fmt.Errorf("%s: 不支持的构建系统 %q（目前只支持 cmake）", spec.Name, spec.Build.System)
	}
	buildRoot := filepath.Join(pkgDir, filepath.FromSlash(spec.Build.Src))
	if !dirExists(buildRoot) {
		return fmt.Errorf("%s: 源码目录不存在: %s", spec.Name, spec.Build.Src)
	}

	buildDir := filepath.Join(buildRoot, "build")
	if !keepBuild {
		if err := os.RemoveAll(buildDir); err != nil {
			return err
		}
	}

	args := []string{"-S", buildRoot, "-B", buildDir, "-DCMAKE_BUILD_TYPE=Release"}
	args = append(args, spec.Build.Args...)
	if spec.Build.CXXFlags != "" {
		// 固定用 -O2 -mtune=generic：绝不能带 -march=native，
		// 否则预编译产物绑死构建机 CPU，换机器可能直接非法指令。
		args = append(args, "-DCMAKE_CXX_FLAGS="+spec.Build.CXXFlags)
	}
	if err := runCmd(buildRoot, "cmake", args...); err != nil {
		return fmt.Errorf("%s: cmake 配置失败: %w", spec.Name, err)
	}
	if err := runCmd(buildRoot, "cmake", "--build", buildDir, "-j"); err != nil {
		return fmt.Errorf("%s: cmake 构建失败: %w", spec.Name, err)
	}
	return nil
}

// stageAndPack 暂存构建产物并打包成 .pm
func stageAndPack(pkgDir string, spec core.BuiltinSpec, packagesDir string) error {
	stageDir, err := os.MkdirTemp("", "pkgtool-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageDir)

	if err := core.StageBuiltinPackage(pkgDir, stageDir, spec); err != nil {
		return fmt.Errorf("%s: 暂存失败: %w", spec.Name, err)
	}

	pm := core.NewPackageManagerWithRepo(packagesDir)
	out := filepath.Join(packagesDir, fmt.Sprintf("%s-%s.pm", spec.Name, spec.Version))
	if _, err := pm.CreatePackageWithOptions(stageDir, core.CreateOptions{OutFile: out, Force: true}); err != nil {
		return fmt.Errorf("%s: 打包失败: %w", spec.Name, err)
	}
	return nil
}

// makeSourceArchive 把「源码 + pkg.json + LICENSE」打成 tar.gz（不含 build/），
// 用于运行时的源码兜底构建。
func makeSourceArchive(pkgDir, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	walkErr := filepath.Walk(pkgDir, func(p string, fi os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		rel, rerr := filepath.Rel(pkgDir, p)
		if rerr != nil {
			return rerr
		}
		if rel == "." {
			return nil
		}
		if fi.IsDir() {
			if fi.Name() == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		hdr, herr := tar.FileInfoHeader(fi, "")
		if herr != nil {
			return herr
		}
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		src, oerr := os.Open(p)
		if oerr != nil {
			return oerr
		}
		defer src.Close()
		_, cerr := io.Copy(tw, src)
		return cerr
	})

	if walkErr == nil {
		walkErr = tw.Close()
	} else {
		tw.Close()
	}
	if cerr := gz.Close(); walkErr == nil {
		walkErr = cerr
	}
	if walkErr != nil {
		os.Remove(dest)
	}
	return walkErr
}

// treeHash 计算包目录内容摘要（跳过 build/），用于构建缓存判断
func treeHash(root string) (string, error) {
	var files []string
	err := filepath.Walk(root, func(p string, fi os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if fi.IsDir() {
			if fi.Name() == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".pm") || strings.HasSuffix(p, ".sha256") {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	h := sha256.New()
	for _, rel := range files {
		data, rerr := os.ReadFile(filepath.Join(root, rel))
		if rerr != nil {
			return "", rerr
		}
		h.Write([]byte(rel))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func stampMatches(stampFile, hash string) bool {
	data, err := os.ReadFile(stampFile)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == hash
}

func loadSpec(file string) (core.BuiltinSpec, error) {
	var spec core.BuiltinSpec
	data, err := os.ReadFile(file)
	if err != nil {
		return spec, err
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		return spec, err
	}
	return spec, nil
}

func loadIndex(file string) core.BuiltinIndex {
	var idx core.BuiltinIndex
	data, err := os.ReadFile(file)
	if err != nil {
		return idx
	}
	if err := json.Unmarshal(data, &idx); err != nil {
		return core.BuiltinIndex{}
	}
	return idx
}

func upsert(idx *core.BuiltinIndex, entry core.BuiltinEntry) {
	for i := range idx.Packages {
		if idx.Packages[i].Spec.Name == entry.Spec.Name {
			idx.Packages[i] = entry
			return
		}
	}
	idx.Packages = append(idx.Packages, entry)
}

func writeIndex(file string, idx core.BuiltinIndex) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(data, '\n'), 0644)
}

func relSlash(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}

func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
