package core

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ===================== 包元数据 =====================

// PackageManifest 描述一个 .pm 包的元数据（包内 manifest.json）
type PackageManifest struct {
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Description string       `json:"description,omitempty"`
	Author      string       `json:"author,omitempty"`
	LibType     LibType      `json:"lib_type,omitempty"`
	LinkLibs    []string     `json:"link_libs,omitempty"`
	Deps        []Dependency `json:"deps,omitempty"`

	// IncludeDirs 包内的头文件目录（相对包根）。
	// 第一个的内容平铺到 extern_inc/<pkg>/；其余保持相对结构，并自动登记为 include 目录。
	// 缺省时按约定探测 include/ 、inc/。
	IncludeDirs []string `json:"include_dirs,omitempty"`

	// LibDirs 包内的库目录（相对包根）。目录下有 debug/、release/ 时按构建模式安装到
	// lib/<mode>/；否则整目录搬到 lib/<pkg>/ 并自动登记为库目录。缺省为 ["lib"]。
	LibDirs []string `json:"lib_dirs,omitempty"`
}

// PackageInfo 本地仓库中一个已存在的包
type PackageInfo struct {
	Manifest PackageManifest
	File     string // .pm 文件绝对路径
	Size     int64
}

// PackageTarget 包安装到项目时的目标目录集合
type PackageTarget struct {
	Root       string
	ExternInc  string
	LibDebug   string
	LibRelease string
	BuildMode  BuildMode
}

// PackageManager 管理本地包仓库。
// 查找分两层：用户仓库（可写，优先）→ 内置仓库（只读，随 pm 分发）。
type PackageManager struct {
	repoDir    string
	builtinDir string
}

// NewPackageManager 创建包管理器实例
func NewPackageManager() *PackageManager {
	return &PackageManager{}
}

// NewPackageManagerWithRepo 使用指定的用户仓库目录，不读写全局配置。
// 适合构建工具（pkgtool）等不需要触碰 ~/.project_manager 的场景。
func NewPackageManagerWithRepo(repoDir string) *PackageManager {
	return &PackageManager{repoDir: repoDir}
}

// Init 初始化仓库目录
func (pm *PackageManager) Init() error {
	pm.repoDir = filepath.Join(ConfigRoot(), "repo")
	return os.MkdirAll(pm.repoDir, 0755)
}

// RepoDir 返回用户仓库目录
func (pm *PackageManager) RepoDir() string {
	return pm.repoDir
}

// SetBuiltinDir 设置只读内置仓库目录（随 pm 分发的基础包）
func (pm *PackageManager) SetBuiltinDir(dir string) {
	pm.builtinDir = dir
}

// BuiltinDir 返回只读内置仓库目录
func (pm *PackageManager) BuiltinDir() string {
	return pm.builtinDir
}

// RepoDirs 返回参与查找的仓库目录，按优先级排序（用户仓库优先）
func (pm *PackageManager) RepoDirs() []string {
	dirs := make([]string, 0, 2)
	if pm.repoDir != "" {
		dirs = append(dirs, pm.repoDir)
	}
	if pm.builtinDir != "" && pm.builtinDir != pm.repoDir {
		dirs = append(dirs, pm.builtinDir)
	}
	return dirs
}

// IsBuiltin 判断某个包文件是否来自只读内置仓库
func (pm *PackageManager) IsBuiltin(file string) bool {
	return pm.isBuiltinFile(file)
}

// isBuiltinFile 判断文件是否位于只读内置仓库中
func (pm *PackageManager) isBuiltinFile(file string) bool {
	if pm.builtinDir == "" {
		return false
	}
	rel, err := filepath.Rel(pm.builtinDir, file)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// ===================== 打包（创建 .pm） =====================

// CreateOptions 打包选项
type CreateOptions struct {
	OutFile string // 输出路径；为空时输出到仓库目录 <name>-<version>.pm
	Force   bool   // 允许覆盖仓库中已存在的同名同版本包
}

// CreatePackage 把一个目录打包成 .pm 文件（tar.gz）。
// srcDir 中应包含 manifest.json 以及 include/、lib/{debug,release}/
// outFile 为空时输出到仓库目录 <name>-<version>.pm。
// 已存在同名同版本包时直接报错，不会静默覆盖。
func (pm *PackageManager) CreatePackage(srcDir, outFile string) (string, error) {
	return pm.CreatePackageWithOptions(srcDir, CreateOptions{OutFile: outFile})
}

// CreatePackageWithOptions 打包并按选项写入。
// 默认拒绝覆盖已存在的 <name>-<version>.pm，保证同一个版本号始终对应同一份内容；
// 只有 Force 为真时才覆盖。
func (pm *PackageManager) CreatePackageWithOptions(srcDir string, opts CreateOptions) (string, error) {
	if pm.repoDir == "" {
		if err := pm.Init(); err != nil {
			return "", err
		}
	}

	outFile := opts.OutFile

	srcDir = filepath.Clean(srcDir)
	info, err := os.Stat(srcDir)
	if err != nil {
		return "", fmt.Errorf("source directory does not exist: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("source is not a directory: %s", srcDir)
	}

	manifest, err := loadManifestFile(filepath.Join(srcDir, "manifest.json"))
	if err != nil {
		// 缺少 manifest.json 时以目录名作为默认包名
		manifest = &PackageManifest{Name: filepath.Base(srcDir), Version: "0.1.0"}
	}
	if strings.TrimSpace(manifest.Name) == "" {
		manifest.Name = filepath.Base(srcDir)
	}
	if strings.TrimSpace(manifest.Version) == "" {
		manifest.Version = "0.1.0"
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}

	if outFile == "" {
		outFile = filepath.Join(pm.repoDir, fmt.Sprintf("%s-%s.pm", manifest.Name, manifest.Version))
	}
	outFile, err = filepath.Abs(outFile)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(outFile), 0755); err != nil {
		return "", err
	}

	// 同一个 name@version 必须是不可变内容，否则依赖不可复现
	if !opts.Force {
		if _, statErr := os.Stat(outFile); statErr == nil {
			return "", fmt.Errorf(
				"package %s@%s already exists in repository: %s (use --force to overwrite)",
				manifest.Name, manifest.Version, outFile)
		}
	}

	tmpFile := outFile + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		return "", err
	}

	writeErr := func() error {
		gz := gzip.NewWriter(f)
		defer gz.Close()
		tw := tar.NewWriter(gz)
		defer tw.Close()

		// 统一写入规范化后的 manifest.json
		rootName := manifest.Name
		if err := writeTarBytes(tw, path.Join(rootName, "manifest.json"), manifestData); err != nil {
			return err
		}

		return filepath.Walk(srcDir, func(p string, fi os.FileInfo, werr error) error {
			if werr != nil {
				return werr
			}
			rel, rerr := filepath.Rel(srcDir, p)
			if rerr != nil {
				return rerr
			}
			if rel == "." {
				return nil
			}
			// 已单独写入
			if filepath.Base(rel) == "manifest.json" && filepath.Dir(rel) == "." {
				return nil
			}

			hdr, herr := tar.FileInfoHeader(fi, "")
			if herr != nil {
				return herr
			}
			hdr.Name = path.Join(rootName, filepath.ToSlash(rel))
			if fi.IsDir() {
				hdr.Name += "/"
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if fi.IsDir() {
				return nil
			}

			src, oerr := os.Open(p)
			if oerr != nil {
				return oerr
			}
			defer src.Close()
			_, cerr := io.Copy(tw, src)
			return cerr
		})
	}()

	closeErr := f.Close()

	if writeErr != nil {
		os.Remove(tmpFile)
		return "", fmt.Errorf("failed to create package: %w", writeErr)
	}
	if closeErr != nil {
		os.Remove(tmpFile)
		return "", closeErr
	}
	if err := os.Rename(tmpFile, outFile); err != nil {
		os.Remove(tmpFile)
		return "", err
	}

	// 记录归档摘要，安装/下载时可据此校验完整性
	if err := writeChecksumSidecar(outFile); err != nil {
		return "", fmt.Errorf("failed to write checksum: %w", err)
	}

	return outFile, nil
}

func writeTarBytes(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0644,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

// ===================== 完整性校验 =====================

// FileSHA256 计算文件的 sha256 摘要（十六进制小写）
func FileSHA256(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ChecksumSidecarPath 返回 .pm 归档对应的摘要边车文件路径
func ChecksumSidecarPath(pmFile string) string {
	return pmFile + ".sha256"
}

// writeChecksumSidecar 把归档摘要写入 <file>.sha256（sha256sum 兼容格式）
func writeChecksumSidecar(pmFile string) error {
	sum, err := FileSHA256(pmFile)
	if err != nil {
		return err
	}
	content := fmt.Sprintf("%s  %s\n", sum, filepath.Base(pmFile))
	return os.WriteFile(ChecksumSidecarPath(pmFile), []byte(content), 0644)
}

// VerifyPackage 校验 .pm 归档的完整性。
// expected 非空时按它校验；为空时读取同名 .sha256 边车文件，边车不存在则跳过校验
// （兼容早期生成的、没有边车的包）。
func VerifyPackage(pmFile, expected string) error {
	if strings.TrimSpace(expected) == "" {
		if data, err := os.ReadFile(ChecksumSidecarPath(pmFile)); err == nil {
			if fields := strings.Fields(string(data)); len(fields) > 0 {
				expected = fields[0]
			}
		}
	}
	if strings.TrimSpace(expected) == "" {
		return nil
	}

	got, err := FileSHA256(pmFile)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, strings.TrimSpace(expected)) {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s",
			filepath.Base(pmFile), got, strings.TrimSpace(expected))
	}
	return nil
}

// ===================== 仓库查询 =====================

// ListPackages 列出所有可见的包（用户仓库 + 内置仓库，同名同版本以用户仓库为准）
func (pm *PackageManager) ListPackages() ([]PackageInfo, error) {
	if pm.repoDir == "" {
		if err := pm.Init(); err != nil {
			return nil, err
		}
	}

	seen := make(map[string]bool)
	var infos []PackageInfo

	for _, dir := range pm.RepoDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".pm" {
				continue
			}
			full := filepath.Join(dir, e.Name())

			m, merr := readManifestFromArchive(full)
			if merr != nil {
				continue // 跳过损坏/非法的包
			}

			key := m.Name + "@" + m.Version
			if seen[key] {
				continue // 前面的仓库优先级更高
			}
			seen[key] = true

			var size int64
			if fi, ierr := e.Info(); ierr == nil {
				size = fi.Size()
			}
			infos = append(infos, PackageInfo{Manifest: *m, File: full, Size: size})
		}
	}

	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Manifest.Name == infos[j].Manifest.Name {
			return compareVersion(infos[i].Manifest.Version, infos[j].Manifest.Version) > 0
		}
		return infos[i].Manifest.Name < infos[j].Manifest.Name
	})

	return infos, nil
}

// SearchPackages 按关键字搜索包（匹配名称/描述）
func (pm *PackageManager) SearchPackages(keyword string) ([]PackageInfo, error) {
	all, err := pm.ListPackages()
	if err != nil {
		return nil, err
	}

	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return all, nil
	}

	var result []PackageInfo
	for _, info := range all {
		if strings.Contains(strings.ToLower(info.Manifest.Name), keyword) ||
			strings.Contains(strings.ToLower(info.Manifest.Description), keyword) {
			result = append(result, info)
		}
	}
	return result, nil
}

// RemovePackage 从用户仓库删除指定包（内置包只读，不允许删除）
func (pm *PackageManager) RemovePackage(name, version string) error {
	file, err := pm.findPackageFile(name, version)
	if err != nil {
		return err
	}
	if pm.isBuiltinFile(file) {
		return fmt.Errorf("package %s is built-in (read-only): %s", name, file)
	}
	return os.Remove(file)
}

// findPackageFile 在仓库中定位包文件。
//
// 匹配以归档内的 manifest.json 为权威元数据，不再从文件名反推 name/version——
// 否则版本号本身含 '-'（如 1.0.0-rc1）时会被切分错误而永远找不到。
func (pm *PackageManager) findPackageFile(name, version string) (string, error) {
	if pm.repoDir == "" {
		if err := pm.Init(); err != nil {
			return "", err
		}
	}

	// name 本身是一个已存在的文件路径
	if fi, err := os.Stat(name); err == nil && !fi.IsDir() {
		return name, nil
	}

	// 快路径：<name>-<version>.pm 直接命中（用户仓库优先）
	if version != "" {
		for _, dir := range pm.RepoDirs() {
			candidate := filepath.Join(dir, fmt.Sprintf("%s-%s.pm", name, version))
			if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
				return candidate, nil
			}
		}
	}

	infos, err := pm.ListPackages()
	if err != nil {
		return "", err
	}

	best := ""
	bestVer := ""
	for _, info := range infos {
		if info.Manifest.Name != name {
			continue
		}
		if version != "" {
			if info.Manifest.Version == version {
				return info.File, nil
			}
			continue
		}
		if best == "" || compareVersion(info.Manifest.Version, bestVer) > 0 {
			best, bestVer = info.File, info.Manifest.Version
		}
	}

	if best != "" {
		return best, nil
	}
	if version != "" {
		return "", fmt.Errorf("package %s@%s not found in repository (%s)", name, version, pm.repoDir)
	}
	return "", fmt.Errorf("package %s not found in repository (%s)", name, pm.repoDir)
}

// ManifestOf 在本地仓库中查找包的元数据（version 为空时取版本最高的一个）。
// 找不到或归档损坏时返回 nil。
func (pm *PackageManager) ManifestOf(name, version string) *PackageManifest {
	file, err := pm.findPackageFile(name, version)
	if err != nil {
		return nil
	}
	m, err := readManifestFromArchive(file)
	if err != nil {
		return nil
	}
	return m
}

// readManifestFromArchive 从 .pm 归档中读取 manifest.json
func readManifestFromArchive(archivePath string) (*PackageManifest, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if path.Base(hdr.Name) != "manifest.json" {
			continue
		}
		data, rerr := io.ReadAll(tr)
		if rerr != nil {
			return nil, rerr
		}
		var m PackageManifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		return &m, nil
	}

	return nil, fmt.Errorf("manifest.json not found in archive: %s", archivePath)
}

// ===================== 安装 =====================

// resolveArchive 定位依赖对应的本地归档。
//
// 常规情况直接命中仓库；如果命中的是内置包但构建环境不匹配，
// 会回退到「用内嵌源码在本机构建」并把结果缓存进用户仓库。
func (pm *PackageManager) resolveArchive(dep Dependency) (string, error) {
	archivePath, err := pm.findPackageFile(dep.Name, dep.Version)
	if err == nil {
		if !pm.isBuiltinFile(archivePath) {
			return archivePath, nil
		}
		entry, ok := BuiltinEntryFor(dep.Name, dep.Version)
		if !ok {
			return archivePath, nil
		}
		if cerr := entry.Compatible(); cerr == nil {
			return archivePath, nil
		} else if built, berr := pm.buildBuiltinLocally(entry); berr == nil {
			return built, nil
		} else {
			return "", fmt.Errorf("内置包 %s@%s 不适用于当前环境（%v），源码兜底构建也失败：%v",
				dep.Name, dep.Version, cerr, berr)
		}
	}

	// 仓库里没有：若是内置包，尝试源码兜底
	if entry, ok := BuiltinEntryFor(dep.Name, dep.Version); ok {
		if built, berr := pm.buildBuiltinLocally(entry); berr == nil {
			return built, nil
		} else {
			return "", fmt.Errorf("%w；内置源码构建也失败：%v", err, berr)
		}
	}
	return "", err
}

// buildBuiltinLocally 用内嵌源码本机构建内置包，产物缓存进用户仓库
func (pm *PackageManager) buildBuiltinLocally(entry BuiltinEntry) (string, error) {
	repo := pm.repoDir
	if repo == "" {
		if err := pm.Init(); err != nil {
			return "", err
		}
		repo = pm.repoDir
	}

	// 用户仓库里已有同版本就不再重复构建
	if p := filepath.Join(repo, fmt.Sprintf("%s-%s.pm", entry.Spec.Name, entry.Spec.Version)); fileExists(p) {
		return p, nil
	}
	return BuildBuiltinFromSource(entry, repo)
}

// InstallResult 描述一次安装实际落地的内容。
// IncludeDirs / LibDirs 是**相对项目根**的目录（如 extern_inc/pkg/extra），
// 调用方需要把它们登记进项目配置，构建时才会被 CMake 使用。
type InstallResult struct {
	Manifest    PackageManifest
	IncludeDirs []string
	LibDirs     []string
}

// Install 把单个包安装到目标目录（头文件 -> extern_inc，库 -> lib/debug|release）
func (pm *PackageManager) Install(dep Dependency, target PackageTarget) (*PackageManifest, error) {
	res, err := pm.InstallOne(dep, target)
	if err != nil {
		return nil, err
	}
	return &res.Manifest, nil
}

// InstallOne 安装单个包，并报告它贡献了哪些额外目录
func (pm *PackageManager) InstallOne(dep Dependency, target PackageTarget) (*InstallResult, error) {
	archivePath, err := pm.resolveArchive(dep)
	if err != nil {
		return nil, err
	}

	// 解压前先校验归档，避免半包/被篡改的包进入项目
	if err := VerifyPackage(archivePath, ""); err != nil {
		return nil, fmt.Errorf("package verification failed: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "pm-install-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	if err := extractArchive(archivePath, tmpDir); err != nil {
		return nil, fmt.Errorf("failed to extract %s: %w", archivePath, err)
	}

	pkgRoot, manifest, err := locatePackageRoot(tmpDir)
	if err != nil {
		return nil, err
	}

	res := &InstallResult{Manifest: *manifest}

	// extern_inc/<pkg> 完全由该包所有：先清空，避免旧版本删掉的头文件残留下来
	pkgIncRoot := filepath.Join(target.ExternInc, manifest.Name)
	if err := os.RemoveAll(pkgIncRoot); err != nil {
		return nil, fmt.Errorf("failed to clean %s: %w", pkgIncRoot, err)
	}

	incDirs, err := resolveIncludeDirs(pkgRoot, manifest.IncludeDirs)
	if err != nil {
		return nil, err
	}
	for i, rel := range incDirs {
		src, err := safeJoin(pkgRoot, rel)
		if err != nil {
			return nil, err
		}
		if !dirExists(src) {
			continue
		}

		if i == 0 {
			// 主头文件目录：内容平铺到 extern_inc/<pkg>/。
			// extern_inc 已在 include 路径上，所以 #include <pkg/xxx.h> 直接可用，无需登记。
			if err := copyDir(src, pkgIncRoot); err != nil {
				return nil, fmt.Errorf("failed to install headers: %w", err)
			}
			continue
		}

		// 额外头文件目录：保持相对结构，安装后必须登记为 include 目录
		dst := filepath.Join(pkgIncRoot, filepath.FromSlash(rel))
		if err := copyDir(src, dst); err != nil {
			return nil, fmt.Errorf("failed to install headers from %s: %w", rel, err)
		}
		res.IncludeDirs = append(res.IncludeDirs, projectRel(target.Root, dst))
	}

	libDirs := manifest.LibDirs
	if len(libDirs) == 0 {
		libDirs = []string{"lib"}
	}
	for _, rel := range libDirs {
		src, err := safeJoin(pkgRoot, rel)
		if err != nil {
			return nil, err
		}
		if !dirExists(src) {
			continue
		}

		debugSrc := filepath.Join(src, "debug")
		releaseSrc := filepath.Join(src, "release")

		if dirExists(debugSrc) || dirExists(releaseSrc) {
			// 约定布局：按构建模式分别落到 lib/debug 与 lib/release，
			// 这两条路径生成器已默认加入 link_directories，无需登记。
			// 某一侧缺失时用另一侧补齐，保证两种模式都能链接。
			if !dirExists(debugSrc) {
				debugSrc = releaseSrc
			}
			if !dirExists(releaseSrc) {
				releaseSrc = debugSrc
			}
			if err := copyDirContents(debugSrc, target.LibDebug); err != nil {
				return nil, fmt.Errorf("failed to install debug libs: %w", err)
			}
			if err := copyDirContents(releaseSrc, target.LibRelease); err != nil {
				return nil, fmt.Errorf("failed to install release libs: %w", err)
			}
			continue
		}

		// 非约定布局（没有 debug/release 子目录）：整目录搬到 lib/<pkg>/ 下并登记
		base := strings.TrimPrefix(filepath.ToSlash(rel), "lib/")
		if base == "lib" || base == "." {
			base = ""
		}
		dst := filepath.Join(target.Root, "lib", manifest.Name, filepath.FromSlash(base))
		if err := copyDir(src, dst); err != nil {
			return nil, fmt.Errorf("failed to install libs from %s: %w", rel, err)
		}
		res.LibDirs = append(res.LibDirs, projectRel(target.Root, dst))
	}

	return res, nil
}

// resolveIncludeDirs 决定用哪些头文件目录：
// manifest 显式声明优先，否则按约定探测 include/ 、inc/
func resolveIncludeDirs(pkgRoot string, declared []string) ([]string, error) {
	if len(declared) > 0 {
		return declared, nil
	}
	for _, candidate := range []string{"include", "inc"} {
		if dirExists(filepath.Join(pkgRoot, candidate)) {
			return []string{candidate}, nil
		}
	}
	return nil, nil
}

// safeJoin 把包内相对路径拼到包根下，并阻止 ../ 逃逸出包
func safeJoin(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute path not allowed in manifest dirs: %s", rel)
	}
	joined := filepath.Join(root, filepath.FromSlash(rel))
	back, err := filepath.Rel(root, joined)
	if err != nil || back == ".." || strings.HasPrefix(back, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes package root: %s", rel)
	}
	return joined, nil
}

// projectRel 返回 abs 相对项目根 root 的斜杠路径
func projectRel(root, abs string) string {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

// installAll 递归安装依赖列表，按 name@version 去重，返回每次安装的结果
func (pm *PackageManager) installAll(deps []Dependency, target PackageTarget) ([]InstallResult, error) {
	installed := make(map[string]bool)
	var results []InstallResult

	var install func(dep Dependency) error
	install = func(dep Dependency) error {
		if strings.TrimSpace(dep.Name) == "" {
			return nil
		}
		key := dep.Name + "@" + dep.Version
		if installed[key] {
			return nil
		}
		installed[key] = true

		res, err := pm.InstallOne(dep, target)
		if err != nil {
			return err
		}
		results = append(results, *res)

		for _, child := range res.Manifest.Deps {
			if err := install(child); err != nil {
				return err
			}
		}
		return nil
	}

	for _, dep := range deps {
		if err := install(dep); err != nil {
			return results, err
		}
	}

	return results, nil
}

// InstallWithDeps 递归安装依赖列表，返回所有被安装包的 manifest
func (pm *PackageManager) InstallWithDeps(deps []Dependency, target PackageTarget) ([]PackageManifest, error) {
	results, err := pm.installAll(deps, target)
	return manifestsOf(results), err
}

// manifestsOf 提取安装结果中的 manifest 列表
func manifestsOf(results []InstallResult) []PackageManifest {
	manifests := make([]PackageManifest, 0, len(results))
	for _, r := range results {
		manifests = append(manifests, r.Manifest)
	}
	return manifests
}

// InstallToProject 把包（含递归依赖）安装到项目，
// 并自动把依赖、链接库、以及包贡献的额外目录登记进项目配置。
func (pm *PackageManager) InstallToProject(p *Project, dep Dependency) ([]PackageManifest, error) {
	if p == nil || !p.IsLoaded() {
		return nil, fmt.Errorf("project not loaded")
	}

	results, err := pm.installAll([]Dependency{dep}, p.PackageTarget())
	manifests := manifestsOf(results)
	if err != nil {
		return manifests, err
	}

	if err := p.applyInstallResults(results, pm); err != nil {
		return manifests, err
	}
	return manifests, nil
}

// InstallProjectDeps 安装项目配置中声明的所有依赖
func (pm *PackageManager) InstallProjectDeps(p *Project) ([]PackageManifest, error) {
	if p == nil || !p.IsLoaded() {
		return nil, fmt.Errorf("project not loaded")
	}

	cfg := p.Config()
	results, err := pm.installAll(cfg.Dependencies, p.PackageTarget())
	manifests := manifestsOf(results)
	if err != nil {
		return manifests, err
	}

	if err := p.applyInstallResults(results, pm); err != nil {
		return manifests, err
	}
	return manifests, nil
}

// ===================== 归档与文件辅助 =====================

// extractArchive 解压 tar.gz 到目标目录，并防止路径穿越
func extractArchive(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		cleanName := filepath.Clean(filepath.FromSlash(hdr.Name))
		if cleanName == "." || cleanName == ".." {
			continue
		}
		target := filepath.Join(destDir, cleanName)
		if !strings.HasPrefix(target, cleanDest) {
			return fmt.Errorf("invalid path in archive: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}

	return nil
}

// locatePackageRoot 在解压目录中定位包含 manifest.json 的包根目录
func locatePackageRoot(root string) (string, *PackageManifest, error) {
	if m, err := loadManifestFile(filepath.Join(root, "manifest.json")); err == nil {
		return root, m, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return "", nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(root, e.Name())
		if m, err := loadManifestFile(filepath.Join(sub, "manifest.json")); err == nil {
			return sub, m, nil
		}
	}

	return "", nil, fmt.Errorf("manifest.json not found in package")
}

// loadManifestFile 读取 manifest.json
func loadManifestFile(file string) (*PackageManifest, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var m PackageManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.Version == "" {
		m.Version = "0.1.0"
	}
	return &m, nil
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// libraryFilePatterns 返回链接库名 lib 在不同平台下可能的库文件名模式。
// manifest 里的 link_libs 是裸名（如 "greeter"），实际文件是 libgreeter.a / libgreeter.so 等。
func libraryFilePatterns(dir, lib string) []string {
	if strings.TrimSpace(dir) == "" || strings.TrimSpace(lib) == "" {
		return nil
	}
	names := []string{filepath.Join(dir, lib), filepath.Join(dir, "lib"+lib)}
	suffixes := []string{".a", ".lib", ".so", ".dylib", ".so.*", ".dll.a"}
	var patterns []string
	for _, n := range names {
		for _, s := range suffixes {
			patterns = append(patterns, n+s)
		}
	}
	return patterns
}

// LibraryFiles 返回 dir 下与链接库名 lib 匹配的库文件（已排序、去重）
func LibraryFiles(dir, lib string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, pattern := range libraryFilePatterns(dir, lib) {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, m := range matches {
			fi, err := os.Stat(m)
			if err != nil || fi.IsDir() || seen[m] {
				continue
			}
			seen[m] = true
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}

// HasLibrary 判断 dir 下是否存在链接库 lib 对应的库文件
func HasLibrary(dir, lib string) bool {
	return len(LibraryFiles(dir, lib)) > 0
}

// containsString 判断字符串切片是否包含某项
func containsString(list []string, item string) bool {
	for _, e := range list {
		if e == item {
			return true
		}
	}
	return false
}

// copyDir 递归复制目录到 dst（dst 作为新目录）
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, p)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)

		if fi.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(p, target, fi.Mode())
	})
}

// copyDirContents 复制目录内容到 dst（不创建 src 这一层）
func copyDirContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
			continue
		}
		fi, err := e.Info()
		if err != nil {
			return err
		}
		if err := copyFile(s, d, fi.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// ===================== 依赖与版本辅助 =====================

// mergeDependency 合并依赖（同名依赖以新版本为准）
func mergeDependency(list []Dependency, dep Dependency) []Dependency {
	for i, d := range list {
		if d.Name == dep.Name {
			list[i] = dep
			return list
		}
	}
	return append(list, dep)
}

// findDependency 在依赖列表中按名字查找，找不到返回 nil
func findDependency(list []Dependency, name string) *Dependency {
	for i := range list {
		if list[i].Name == name {
			return &list[i]
		}
	}
	return nil
}

// removeDependency 从依赖列表移除
func removeDependency(list []Dependency, name string) []Dependency {
	result := make([]Dependency, 0, len(list))
	for _, d := range list {
		if d.Name != name {
			result = append(result, d)
		}
	}
	return result
}

// appendUnique 去重追加字符串
func appendUnique(list []string, items ...string) []string {
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			continue
		}
		exists := false
		for _, e := range list {
			if e == item {
				exists = true
				break
			}
		}
		if !exists {
			list = append(list, item)
		}
	}
	return list
}

// removeString 从字符串列表中移除指定项
func removeString(list []string, item string) []string {
	result := make([]string, 0, len(list))
	for _, e := range list {
		if e != item {
			result = append(result, e)
		}
	}
	return result
}

// compareVersion 简单版本比较：a>b 返回 1，a<b 返回 -1，相等返回 0
func compareVersion(a, b string) int {
	as := strings.Split(strings.TrimSpace(a), ".")
	bs := strings.Split(strings.TrimSpace(b), ".")

	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}

	for i := 0; i < n; i++ {
		var av, bv string
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}

		ai, aerr := strconv.Atoi(av)
		bi, berr := strconv.Atoi(bv)

		if aerr == nil && berr == nil {
			if ai != bi {
				if ai > bi {
					return 1
				}
				return -1
			}
			continue
		}

		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}

	return 0
}
