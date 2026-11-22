package core

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BuildMode 编译模式
type BuildMode string

const (
	BuildModeDebug   BuildMode = "debug"
	BuildModeRelease BuildMode = "release"
)

// GenerateType 生成文件类型
type GenerateType string

const (
	GenerateTypeExecutable GenerateType = "executable"
	GenerateTypeStaticLib  GenerateType = "static_lib"
	GenerateTypeSharedLib  GenerateType = "share_lib"
)

// LibType 库类型
type LibType string

const (
	LibTypeStatic LibType = "static"
	LibTypeShared LibType = "shared"
)

// ===================== 依赖项 =====================

// Dependency 项目依赖的包
type Dependency struct {
	Name    string  `json:"name"`
	Version string  `json:"version"`
	LibType LibType `json:"lib_type,omitempty"`
}

// ProjectDependencies 项目依赖集合
type ProjectDependencies struct {
	Libs    []string `json:"libs"`
	LibDirs []string `json:"lib_dirs"`
	IncDirs []string `json:"inc_dirs"`
	SrcDirs []string `json:"src_dirs"`
}

// ===================== 构建选项 =====================

// BuildModeOptions 单个构建模式的选项
type BuildModeOptions struct {
	Defines      []string `json:"defines"`
	Optimization string   `json:"optimization"`
}

// BuildOptions 构建选项
type BuildOptions struct {
	Debug   BuildModeOptions `json:"debug"`
	Release BuildModeOptions `json:"release"`
}

// ===================== 构建目标 =====================

// Target 构建目标定义
type Target struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // "executable" or "library"
	SourceFiles []string `json:"source_files,omitempty"`
	LinkLibs    []string `json:"link_libraries,omitempty"`
}

// ===================== 项目配置 =====================

// ProjectConfig 项目核心配置（对应 config/.pmproject）
type ProjectConfig struct {
	Name             string              `json:"name"`
	Version          string              `json:"version"`
	Path             string              `json:"path,omitempty"`
	UUID             string              `json:"uuid"`
	BuildSystem      string              `json:"build_system"`
	CMakeMinimum     string              `json:"cmake_minimum_version"`
	Language         string              `json:"language"`
	CXXStandard      int                 `json:"cxx_standard"`
	SourceDirs       []string            `json:"source_dirs"`
	IncludeDirs      []string            `json:"include_dirs"`
	ExternIncDir     string              `json:"extern_inc_dir"`
	LibDir           string              `json:"lib_dir"`
	OutputDir        string              `json:"output_dir"`
	MainFile         string              `json:"main_file,omitempty"`
	RunArgs          string              `json:"run_args,omitempty"`
	BuildMode        BuildMode           `json:"build_mode"`
	GenerateType     GenerateType        `json:"generate_type"`
	Dependencies     []Dependency        `json:"dependencies"`
	LegacyDeps       ProjectDependencies `json:"deps,omitempty"`
	BuildOptions     BuildOptions        `json:"build_options"`
	Targets          []Target            `json:"targets"`
	ExtraIncludeDirs []string            `json:"extra_include_dirs,omitempty"`
	ExtraLibDirs     []string            `json:"extra_lib_dirs,omitempty"`
}

// ===================== 项目目录信息 =====================

// ProjectDirInfo 标准项目目录布局
type ProjectDirInfo struct {
	Root             string
	Config           string
	Doc              string
	ExternInc        string
	Inc              string
	LibDebug         string
	LibRelease       string
	Main             string
	OutputDebugBin   string
	OutputDebugLib   string
	OutputReleaseBin string
	OutputReleaseLib string
	Src              string
}

// ===================== Project =====================

// Project 表示单个 C++ 项目的生命周期管理
type Project struct {
	cfg     ProjectConfig
	dirInfo ProjectDirInfo
	loaded  bool
}

// NewProject 创建空的 Project 实例
func NewProject() *Project {
	return &Project{}
}

// Config 获取项目配置
func (p *Project) Config() ProjectConfig {
	return p.cfg
}

// DirInfo 获取目录信息
func (p *Project) DirInfo() ProjectDirInfo {
	return p.dirInfo
}

// IsLoaded 是否已加载
func (p *Project) IsLoaded() bool {
	return p.loaded
}

// PackageTarget 返回当前项目的包安装目标目录
func (p *Project) PackageTarget() PackageTarget {
	return PackageTarget{
		Root:       p.dirInfo.Root,
		ExternInc:  p.dirInfo.ExternInc,
		LibDebug:   p.dirInfo.LibDebug,
		LibRelease: p.dirInfo.LibRelease,
		BuildMode:  p.cfg.BuildMode,
	}
}

// AddDependency 向项目配置添加/更新依赖并保存
func (p *Project) AddDependency(dep Dependency) error {
	cfg := p.Config()
	cfg.Dependencies = mergeDependency(cfg.Dependencies, dep)
	return p.SaveConfig(cfg)
}

// RemoveDependencyByName 从项目配置移除依赖（同时移除同名链接库）
func (p *Project) RemoveDependencyByName(name string) error {
	cfg := p.Config()
	cfg.Dependencies = removeDependency(cfg.Dependencies, name)
	cfg.LegacyDeps.Libs = removeString(cfg.LegacyDeps.Libs, name)
	return p.SaveConfig(cfg)
}

// UninstallDependency 卸载依赖：移除配置声明，并清理安装时复制进项目的文件：
//   - extern_inc/<pkg>/（含登记的额外头文件目录）
//   - lib/<pkg>/（非约定布局的额外库目录）
//   - lib/<mode>/ 下由该包 link_libs 引入的库文件
//   - deps.inc_dirs / deps.lib_dirs / deps.libs 中属于该包的条目
//
// 仍被其它依赖声明的库会保留；某个剩余依赖的元数据已不可获取时保守处理，不删库文件。
// 返回被删除的路径列表（用于向用户展示）。
func (p *Project) UninstallDependency(name string) ([]string, error) {
	if !p.loaded {
		return nil, fmt.Errorf("project not loaded")
	}

	cfg := p.Config()
	target := findDependency(cfg.Dependencies, name)
	if target == nil {
		return nil, fmt.Errorf("dependency %s not found", name)
	}

	pm := NewPackageManager()

	// 该包引入的链接库名（元数据取不到时退回包名，符合 lib<name> 惯例）
	removedLibs := []string{name}
	if m := pm.ManifestOf(target.Name, target.Version); m != nil && len(m.LinkLibs) > 0 {
		removedLibs = m.LinkLibs
	}

	// 移除依赖声明、不再需要的库名与目录条目
	cfg.Dependencies = removeDependency(cfg.Dependencies, name)
	cfg.LegacyDeps.Libs = removeString(cfg.LegacyDeps.Libs, name)
	orphanLibs := p.staleLibs(&cfg, name, removedLibs, nil, pm)
	removed := p.prunePackageDirs(&cfg, name, nil)

	if err := p.SaveConfig(cfg); err != nil {
		return removed, err
	}

	// 头文件目录
	incDir := filepath.Join(p.dirInfo.ExternInc, name)
	if fi, err := os.Stat(incDir); err == nil && fi.IsDir() {
		if err := os.RemoveAll(incDir); err != nil {
			return removed, fmt.Errorf("failed to remove installed headers %s: %w", incDir, err)
		}
		removed = append(removed, incDir)
	}

	// 只属于该包的库文件
	for _, lib := range orphanLibs {
		for _, dir := range []string{p.dirInfo.LibDebug, p.dirInfo.LibRelease} {
			for _, file := range LibraryFiles(dir, lib) {
				if err := os.Remove(file); err != nil {
					return removed, fmt.Errorf("failed to remove library %s: %w", file, err)
				}
				removed = append(removed, file)
			}
		}
	}

	return removed, nil
}

// applyInstallResults 把安装结果登记进项目配置。
// 升级（或重装）时先清理旧版本留下的库文件与目录条目，避免旧内容残留。
func (p *Project) applyInstallResults(results []InstallResult, pm *PackageManager) error {
	cfg := p.Config()

	for _, res := range results {
		name := res.Manifest.Name

		// 升级：旧版本声明的库名，若新版本不再声明、其它依赖也不需要，则清理
		if old := findDependency(cfg.Dependencies, name); old != nil {
			oldLibs := []string{name}
			if m := pm.ManifestOf(old.Name, old.Version); m != nil && len(m.LinkLibs) > 0 {
				oldLibs = m.LinkLibs
			}
			orphans := p.staleLibs(&cfg, name, oldLibs, res.Manifest.LinkLibs, pm)
			for _, lib := range orphans {
				for _, dir := range []string{p.dirInfo.LibDebug, p.dirInfo.LibRelease} {
					for _, file := range LibraryFiles(dir, lib) {
						os.Remove(file)
					}
				}
			}
		}

		cfg.Dependencies = mergeDependency(cfg.Dependencies, Dependency{
			Name:    name,
			Version: res.Manifest.Version,
			LibType: res.Manifest.LibType,
		})

		// 旧版本登记、新版本不再提供的额外目录要清掉
		keepDirs := make([]string, 0, len(res.IncludeDirs)+len(res.LibDirs))
		keepDirs = append(keepDirs, res.IncludeDirs...)
		keepDirs = append(keepDirs, res.LibDirs...)
		p.prunePackageDirs(&cfg, name, keepDirs)

		cfg.LegacyDeps.Libs = appendUnique(cfg.LegacyDeps.Libs, res.Manifest.LinkLibs...)
		cfg.LegacyDeps.IncDirs = appendUnique(cfg.LegacyDeps.IncDirs, res.IncludeDirs...)
		cfg.LegacyDeps.LibDirs = appendUnique(cfg.LegacyDeps.LibDirs, res.LibDirs...)
	}

	return p.SaveConfig(cfg)
}

// staleLibs 返回「旧版本引入、但新版本不再需要、其它依赖也不需要」的库名，
// 并把它们从 deps.libs 中移除。newLibs 为 nil 表示该包被整体卸载。
func (p *Project) staleLibs(cfg *ProjectConfig, pkgName string, oldLibs, newLibs []string, pm *PackageManager) []string {
	keep := make(map[string]bool, len(newLibs))
	for _, lib := range newLibs {
		keep[lib] = true
	}

	// 其它依赖仍然需要的库名
	uncertain := false
	for _, d := range cfg.Dependencies {
		if d.Name == pkgName {
			continue
		}
		m := pm.ManifestOf(d.Name, d.Version)
		if m == nil {
			uncertain = true
			continue
		}
		for _, lib := range m.LinkLibs {
			keep[lib] = true
		}
	}
	if uncertain {
		// 无法确认剩余依赖需要什么，配置里现有的库名一律保留
		for _, lib := range cfg.LegacyDeps.Libs {
			keep[lib] = true
		}
	}

	var stale []string
	for _, lib := range oldLibs {
		if keep[lib] {
			continue
		}
		stale = append(stale, lib)
		cfg.LegacyDeps.Libs = removeString(cfg.LegacyDeps.Libs, lib)
	}
	return stale
}

// prunePackageDirs 移除属于该包、但不在 keep 中的 extra 目录（连同磁盘目录），
// 返回被删除的路径。keep 为空表示该包的目录全部移除（卸载场景）。
//
// 约定：包引入的目录一律位于 extern_inc/<pkg>/ 或 lib/<pkg>/ 之下，据此识别归属；
// 最后顺带清理空的容器目录，避免留下 lib/<pkg>/ 这种空壳。
func (p *Project) prunePackageDirs(cfg *ProjectConfig, name string, keep []string) []string {
	keepSet := make(map[string]bool, len(keep))
	for _, d := range keep {
		keepSet[filepath.ToSlash(d)] = true
	}

	prefixes := []string{
		filepath.Join(p.dirInfo.ExternInc, name) + string(os.PathSeparator),
		filepath.Join(p.dirInfo.Root, "lib", name) + string(os.PathSeparator),
	}

	var removed []string
	filter := func(list []string) []string {
		kept := make([]string, 0, len(list))
		for _, dir := range list {
			abs := filepath.Join(p.dirInfo.Root, filepath.FromSlash(dir))
			owned := false
			for _, prefix := range prefixes {
				if strings.HasPrefix(abs, prefix) {
					owned = true
					break
				}
			}
			if !owned || keepSet[filepath.ToSlash(dir)] {
				kept = append(kept, dir)
				continue
			}
			if err := os.RemoveAll(abs); err == nil {
				removed = append(removed, abs)
			}
		}
		return kept
	}

	cfg.LegacyDeps.IncDirs = filter(cfg.LegacyDeps.IncDirs)
	cfg.LegacyDeps.LibDirs = filter(cfg.LegacyDeps.LibDirs)

	if dir := filepath.Join(p.dirInfo.Root, "lib", name); dirExists(dir) {
		if err := os.Remove(dir); err == nil {
			removed = append(removed, dir)
		}
	}

	return removed
}

// DependencyStatus 返回依赖在项目中的安装状态：
// installed / headers-only / libs-only / missing。
// 按该依赖自己的文件判断，库文件名以包 manifest 的 link_libs 为准（取不到时退回包名）。
func (p *Project) DependencyStatus(pm *PackageManager, d Dependency) string {
	hasHeaders := false
	if fi, err := os.Stat(filepath.Join(p.dirInfo.ExternInc, d.Name)); err == nil && fi.IsDir() {
		hasHeaders = true
	}

	libNames := []string{d.Name}
	if pm != nil {
		if m := pm.ManifestOf(d.Name, d.Version); m != nil && len(m.LinkLibs) > 0 {
			libNames = m.LinkLibs
		}
	}

	hasLibs := false
	for _, lib := range libNames {
		if HasLibrary(p.dirInfo.LibDebug, lib) || HasLibrary(p.dirInfo.LibRelease, lib) {
			hasLibs = true
			break
		}
	}

	switch {
	case hasHeaders && hasLibs:
		return "installed"
	case hasHeaders:
		return "headers-only"
	case hasLibs:
		return "libs-only"
	default:
		return "missing"
	}
}

// AddLinkLibs 向项目配置追加需要链接的库（去重）
func (p *Project) AddLinkLibs(libs ...string) error {
	cfg := p.Config()
	cfg.LegacyDeps.Libs = appendUnique(cfg.LegacyDeps.Libs, libs...)
	return p.SaveConfig(cfg)
}

// RemoveLinkLibs 从项目配置移除链接库
func (p *Project) RemoveLinkLibs(libs ...string) error {
	cfg := p.Config()
	for _, l := range libs {
		cfg.LegacyDeps.Libs = removeString(cfg.LegacyDeps.Libs, l)
	}
	return p.SaveConfig(cfg)
}

// fillDirInfo 填充目录信息
func (p *Project) fillDirInfo(projPath string) {
	p.dirInfo = ProjectDirInfo{
		Root:             projPath,
		Config:           filepath.Join(projPath, "config"),
		Doc:              filepath.Join(projPath, "doc"),
		ExternInc:        filepath.Join(projPath, "extern_inc"),
		Inc:              filepath.Join(projPath, "inc"),
		LibDebug:         filepath.Join(projPath, "lib", "debug"),
		LibRelease:       filepath.Join(projPath, "lib", "release"),
		Main:             filepath.Join(projPath, "main"),
		OutputDebugBin:   filepath.Join(projPath, "output", "debug", "bin"),
		OutputDebugLib:   filepath.Join(projPath, "output", "debug", "lib"),
		OutputReleaseBin: filepath.Join(projPath, "output", "release", "bin"),
		OutputReleaseLib: filepath.Join(projPath, "output", "release", "lib"),
		Src:              filepath.Join(projPath, "src"),
	}
}

// configFilePath 返回配置文件路径 config/.pmproject
func (p *Project) configFilePath() string {
	return filepath.Join(p.dirInfo.Config, ".pmproject")
}

// generateUUID 生成一个简单的 UUID v4 风格字符串
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// DefaultConfig 创建默认配置
func DefaultConfig(name, path string) ProjectConfig {
	return ProjectConfig{
		Name:         name,
		Version:      "0.1.0",
		Path:         path,
		UUID:         generateUUID(),
		BuildSystem:  "cmake",
		CMakeMinimum: "3.10",
		Language:     "CXX",
		CXXStandard:  17,
		SourceDirs:   []string{"src"},
		IncludeDirs:  []string{"inc"},
		ExternIncDir: "extern_inc",
		LibDir:       "lib",
		OutputDir:    "output",
		MainFile:     "main.cc",
		BuildMode:    BuildModeDebug,
		GenerateType: GenerateTypeExecutable,
		Dependencies: []Dependency{},
		BuildOptions: BuildOptions{
			Debug: BuildModeOptions{
				Defines:      []string{"DEBUG"},
				Optimization: "-O0",
			},
			Release: BuildModeOptions{
				Defines:      []string{"NDEBUG"},
				Optimization: "-O3",
			},
		},
		Targets: []Target{
			{
				Name:        name,
				Type:        "executable",
				SourceFiles: []string{"main/main.cc"},
			},
		},
	}
}

// ===================== 创建项目 =====================

// CreateProject 在指定路径下创建项目骨架和默认配置
func CreateProject(projName, parentPath string) (string, error) {
	cleanPath := strings.TrimRight(parentPath, "/")
	if cleanPath == "" {
		cleanPath = "."
	}

	// 统一使用绝对路径，保证项目路径与当前工作目录无关
	absParent, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve parent directory: %w", err)
	}
	fullPath := filepath.Join(absParent, projName)

	if err := os.MkdirAll(absParent, 0755); err != nil {
		return "", fmt.Errorf("failed to create parent directory: %w", err)
	}

	if _, err := os.Stat(fullPath); err == nil {
		return "", fmt.Errorf("project directory already exists: %s", fullPath)
	}

	proj := NewProject()
	proj.fillDirInfo(fullPath)

	if err := proj.createDirStructure(); err != nil {
		return "", fmt.Errorf("failed to create directory structure: %w", err)
	}

	// 生成默认入口文件，保证新建项目开箱即可构建
	if err := proj.writeMainTemplate(); err != nil {
		return "", fmt.Errorf("failed to write default main file: %w", err)
	}

	proj.cfg = DefaultConfig(projName, fullPath)

	if err := proj.writeConfig(); err != nil {
		return "", fmt.Errorf("failed to write config: %w", err)
	}

	return fullPath, nil
}

// createDirStructure 创建标准目录结构
func (p *Project) createDirStructure() error {
	dirs := []string{
		p.dirInfo.Config,
		p.dirInfo.Doc,
		p.dirInfo.ExternInc,
		p.dirInfo.Inc,
		p.dirInfo.Main,
		p.dirInfo.Src,
		p.dirInfo.LibDebug,
		p.dirInfo.LibRelease,
		p.dirInfo.OutputDebugBin,
		p.dirInfo.OutputDebugLib,
		p.dirInfo.OutputReleaseBin,
		p.dirInfo.OutputReleaseLib,
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", d, err)
		}
	}
	return nil
}

// writeMainTemplate 写入默认的 main/main.cc（若不存在）
func (p *Project) writeMainTemplate() error {
	mainFile := filepath.Join(p.dirInfo.Main, "main.cc")
	if _, err := os.Stat(mainFile); err == nil {
		return nil // 已存在，不覆盖
	}

	content := fmt.Sprintf(`#include <iostream>

int main(int argc, char* argv[]) {
    std::cout << "Hello from %s!" << std::endl;
    return 0;
}
`, filepath.Base(p.dirInfo.Root))

	return os.WriteFile(mainFile, []byte(content), 0644)
}

// ===================== 加载项目 =====================

// LoadProject 从已有项目路径加载
func LoadProject(projPath string) (*Project, error) {
	info, err := os.Stat(projPath)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("project path does not exist: %s", projPath)
	}

	proj := NewProject()
	proj.fillDirInfo(projPath)

	cfgPath := proj.configFilePath()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	cfg.Path = projPath
	proj.cfg = cfg
	proj.loaded = true

	return proj, nil
}

// ===================== 保存配置 =====================

// SaveConfig 保存配置到文件
func (p *Project) SaveConfig(cfg ProjectConfig) error {
	p.cfg = cfg
	return p.writeConfig()
}

func (p *Project) writeConfig() error {
	if err := os.MkdirAll(p.dirInfo.Config, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(p.cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(p.configFilePath(), data, 0644)
}

// ===================== 构建 =====================

// Build 构建项目（cmake 输出直接打印到当前进程的标准输出）
func (p *Project) Build(rebuild bool) error {
	return p.BuildWithWriter(rebuild, os.Stdout, os.Stderr)
}

// BuildWithWriter 构建项目，并把 cmake 的输出写入 out/errW。
// 构建过程不会修改当前进程的工作目录，可安全地在后台 goroutine 中执行。
func (p *Project) BuildWithWriter(rebuild bool, out, errW io.Writer) error {
	if !p.loaded {
		return fmt.Errorf("project not loaded")
	}
	if out == nil {
		out = io.Discard
	}
	if errW == nil {
		errW = io.Discard
	}

	cmakeContent := GenerateCMakeLists(p.cfg, p.dirInfo)
	cmakePath := filepath.Join(p.dirInfo.Root, "CMakeLists.txt")
	if err := os.WriteFile(cmakePath, []byte(cmakeContent), 0644); err != nil {
		return fmt.Errorf("failed to write CMakeLists.txt: %w", err)
	}

	buildDir := filepath.Join(p.dirInfo.Root, "build")
	if rebuild {
		os.RemoveAll(buildDir)
	}

	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return err
	}

	buildType := "Debug"
	if p.cfg.BuildMode == BuildModeRelease {
		buildType = "Release"
	}

	cmakeCmd := fmt.Sprintf("cmake -DCMAKE_BUILD_TYPE=%s ..", buildType)
	if err := runShellCommand(cmakeCmd, buildDir, out, errW); err != nil {
		return fmt.Errorf("cmake configuration failed: %w", err)
	}

	if err := runShellCommand("cmake --build .", buildDir, out, errW); err != nil {
		return fmt.Errorf("cmake build failed: %w", err)
	}

	return nil
}

// BuildCapture 构建项目并返回合并后的构建日志
func (p *Project) BuildCapture(rebuild bool) (string, error) {
	var buf bytes.Buffer
	err := p.BuildWithWriter(rebuild, &buf, &buf)
	return buf.String(), err
}

// ===================== 运行 =====================

// Run 运行编译后的程序
func (p *Project) Run() error {
	cmd, err := p.RunCommand()
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// RunCommand 返回运行当前构建模式下可执行文件的命令。
// 标准输入输出保持未设置，调用方可自行接管
// （例如 TUI 通过 tea.ExecProcess 把终端交给子进程）。
func (p *Project) RunCommand() (*exec.Cmd, error) {
	if !p.loaded {
		return nil, fmt.Errorf("project not loaded")
	}

	exePath := p.ResolveExecutable()
	if exePath == "" {
		binDir := p.dirInfo.OutputDebugBin
		if p.cfg.BuildMode == BuildModeRelease {
			binDir = p.dirInfo.OutputReleaseBin
		}
		return nil, fmt.Errorf("executable not found in %s, build the project first", binDir)
	}

	cmd := exec.Command(exePath, strings.Fields(p.cfg.RunArgs)...)
	cmd.Dir = p.dirInfo.Root

	return cmd, nil
}

// ResolveExecutable 定位当前构建模式下的可执行文件，找不到返回空串
func (p *Project) ResolveExecutable() string {
	binDir := p.dirInfo.OutputDebugBin
	if p.cfg.BuildMode == BuildModeRelease {
		binDir = p.dirInfo.OutputReleaseBin
	}

	// 优先按名称（含 debug 的 d 后缀）查找
	for _, name := range []string{p.cfg.Name, p.cfg.Name + "d"} {
		full := filepath.Join(binDir, name)
		if fi, err := os.Stat(full); err == nil && !fi.IsDir() {
			return full
		}
	}

	// 回退：目录下第一个可执行文件
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if fi.Mode()&0111 != 0 {
			return filepath.Join(binDir, e.Name())
		}
	}

	return ""
}

// ===================== 清理 =====================

// Clean 清理编译产物
func (p *Project) Clean() error {
	if !p.loaded {
		return fmt.Errorf("project not loaded")
	}

	if p.dirInfo.Root == "" || p.dirInfo.Root == "/" {
		return fmt.Errorf("invalid project root for clean")
	}

	outputDir := filepath.Join(p.dirInfo.Root, "output")
	os.RemoveAll(outputDir)
	os.MkdirAll(p.dirInfo.OutputDebugBin, 0755)
	os.MkdirAll(p.dirInfo.OutputDebugLib, 0755)
	os.MkdirAll(p.dirInfo.OutputReleaseBin, 0755)
	os.MkdirAll(p.dirInfo.OutputReleaseLib, 0755)

	buildDir := filepath.Join(p.dirInfo.Root, "build")
	os.RemoveAll(buildDir)

	return nil
}

// ===================== Main 文件扫描 =====================

// ScanMainFiles 扫描 main/ 目录下的可用源文件
func (p *Project) ScanMainFiles() ([]string, error) {
	entries, err := os.ReadDir(p.dirInfo.Main)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".cc" || ext == ".cpp" || ext == ".cxx" || ext == ".c" {
			files = append(files, name)
		}
	}

	return files, nil
}

// ===================== 辅助函数 =====================

// runShellCommand 在 dir 目录下运行 shell 命令，输出写入 out/errW
func runShellCommand(cmdStr, dir string, out, errW io.Writer) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell, "-c", cmdStr)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = errW
	cmd.Stdin = nil

	return cmd.Run()
}
