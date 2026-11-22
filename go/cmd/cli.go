package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rjf12138/project_manager/core"
)

// ===================== 通用辅助 =====================

// mustSolution 初始化 Solution 与包管理器
func mustSolution() (*core.Solution, *core.PackageManager, error) {
	s := core.NewSolution()
	if err := s.Init(); err != nil {
		return nil, nil, err
	}

	pm := core.NewPackageManager()
	if err := pm.Init(); err != nil {
		return nil, nil, err
	}

	// 释放并挂载随二进制分发的内置包（只读层，用户仓库优先）
	if dir, err := core.MaterializeBuiltin(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: 内置包释放失败: %v\n", err)
	} else if dir != "" {
		pm.SetBuiltinDir(dir)
	}

	return s, pm, nil
}

// newFlagSet 创建统一的 FlagSet
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

// firstArg 返回第一个位置参数，没有则返回空串
func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// indexOf 返回目标字符串在切片中的下标，找不到返回 -1
func indexOf(args []string, target string) int {
	for i, a := range args {
		if a == target {
			return i
		}
	}
	return -1
}

// normalizeArgs 把 flag 与位置参数分离，允许两者以任意顺序出现。
// valueFlags 指定需要额外取值的 flag（如 -t/-p/-o），返回 flags 与 positional。
func normalizeArgs(args []string, valueFlags map[string]bool) (flags []string, positional []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]

		if a == "--" {
			// 分隔符之后的全部作为位置参数
			positional = append(positional, args[i+1:]...)
			return flags, positional
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positional = append(positional, a)
			continue
		}

		flags = append(flags, a)

		name := a
		if eq := strings.Index(a, "="); eq >= 0 {
			name = a[:eq]
		}
		if valueFlags[name] && !strings.Contains(a, "=") && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return flags, positional
}

// resolveProject 解析项目：name 为空用活动项目，否则按名称或路径定位
func resolveProject(s *core.Solution, name string) (*core.Project, error) {
	if strings.TrimSpace(name) == "" {
		proj := s.ActiveProject()
		if proj == nil {
			return nil, fmt.Errorf("no active project; specify a project name or run 'pm load <path>' first")
		}
		return proj, nil
	}

	if proj := s.FindProjectByName(name); proj != nil {
		return proj, nil
	}

	// 尝试把参数当作项目路径加载
	if abs, err := filepath.Abs(name); err == nil {
		if err := s.LoadProject(abs); err == nil {
			return s.GetProject(abs), nil
		}
	}

	return nil, fmt.Errorf("project not found: %s", name)
}

// parsePkgSpec 解析 <name> 或 <name>@<version>
func parsePkgSpec(spec string) core.Dependency {
	spec = strings.TrimSpace(spec)
	if i := strings.LastIndex(spec, "@"); i > 0 {
		return core.Dependency{Name: spec[:i], Version: spec[i+1:]}
	}
	return core.Dependency{Name: spec}
}

// ===================== 项目管理 =====================

// cmdInit 实现 pm init <name> [parent-path]
func cmdInit(args []string) error {
	fs := newFlagSet("init")
	parent := fs.String("path", ".", "parent directory of the new project")
	fs.StringVar(parent, "p", ".", "parent directory of the new project")

	flags, positional := normalizeArgs(args, map[string]bool{"-path": true, "--path": true, "-p": true})
	if err := fs.Parse(flags); err != nil {
		return err
	}

	if len(positional) < 1 {
		return fmt.Errorf("usage: pm init <name> [parent-path]")
	}
	name := positional[0]
	parentPath := *parent
	if len(positional) > 1 {
		parentPath = positional[1]
	}

	s, _, err := mustSolution()
	if err != nil {
		return err
	}

	if err := s.CreateProject(name, parentPath); err != nil {
		return err
	}

	proj := s.ActiveProject()
	fmt.Printf("Created project %q at %s\n", name, proj.Config().Path)
	return nil
}

// cmdLoad 实现 pm load <path>
func cmdLoad(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: pm load <project-path>")
	}

	abs, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	s, _, err := mustSolution()
	if err != nil {
		return err
	}

	if err := s.LoadProject(abs); err != nil {
		return err
	}

	fmt.Printf("Loaded project %q (%s)\n", s.ProjectNameFromPath(abs), abs)
	return nil
}

// cmdList 实现 pm list
func cmdList(args []string) error {
	s, _, err := mustSolution()
	if err != nil {
		return err
	}

	paths := s.ProjectPaths()
	if len(paths) == 0 {
		fmt.Println("No projects registered. Use 'pm init <name>' or 'pm load <path>'.")
		return nil
	}

	active := s.ActiveProjectPath()
	fmt.Printf("Projects (%d):\n", len(paths))
	for _, p := range paths {
		marker := "  "
		if p == active {
			marker = "* "
		}

		mode := "-"
		if proj := s.GetProject(p); proj != nil {
			mode = string(proj.Config().BuildMode)
		}

		fmt.Printf("%s%-20s %-8s %s\n", marker, s.ProjectNameFromPath(p), mode, p)
	}
	return nil
}

// cmdRemove 实现 pm remove <name|path>（仅从列表移除，不删除文件）
func cmdRemove(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: pm remove <name|path>")
	}

	s, _, err := mustSolution()
	if err != nil {
		return err
	}

	path := s.FindProjectPathByName(args[0])
	if path == "" {
		return fmt.Errorf("project not found: %s", args[0])
	}

	name := s.ProjectNameFromPath(path)
	if err := s.CloseProject(path); err != nil {
		return err
	}

	fmt.Printf("Removed project %q from list (files untouched)\n", name)
	return nil
}

// ===================== 构建 / 运行 =====================

// cmdBuild 实现 pm build [name] [-t debug|release] [-r]
func cmdBuild(args []string) error {
	fs := newFlagSet("build")
	mode := fs.String("t", "", "build mode: debug or release")
	fs.StringVar(mode, "type", "", "build mode: debug or release")
	rebuild := fs.Bool("r", false, "clean before building")
	fs.BoolVar(rebuild, "rebuild", false, "clean before building")

	flags, positional := normalizeArgs(args, map[string]bool{"-t": true, "--type": true})
	if err := fs.Parse(flags); err != nil {
		return err
	}

	s, _, err := mustSolution()
	if err != nil {
		return err
	}

	proj, err := resolveProject(s, firstArg(positional))
	if err != nil {
		return err
	}

	if *mode != "" {
		cfg := proj.Config()
		switch strings.ToLower(*mode) {
		case "debug":
			cfg.BuildMode = core.BuildModeDebug
		case "release":
			cfg.BuildMode = core.BuildModeRelease
		default:
			return fmt.Errorf("invalid build mode %q (expected debug or release)", *mode)
		}
		if err := proj.SaveConfig(cfg); err != nil {
			return err
		}
	}

	cfg := proj.Config()
	s.SetActiveProject(cfg.Path)

	fmt.Printf("Building %s [%s]%s...\n", cfg.Name, cfg.BuildMode, map[bool]string{true: " (clean)", false: ""}[*rebuild])
	if err := proj.Build(*rebuild); err != nil {
		return err
	}

	fmt.Printf("Build succeeded: %s\n", proj.ResolveExecutable())
	return nil
}

// cmdClean 实现 pm clean [name]
func cmdClean(args []string) error {
	s, _, err := mustSolution()
	if err != nil {
		return err
	}

	proj, err := resolveProject(s, firstArg(args))
	if err != nil {
		return err
	}

	if err := proj.Clean(); err != nil {
		return err
	}

	fmt.Printf("Cleaned build artifacts of %s\n", proj.Config().Name)
	return nil
}

// cmdRun 实现 pm run [name] [-- program-args]
func cmdRun(args []string) error {
	var name string
	var programArgs []string

	if i := indexOf(args, "--"); i >= 0 {
		if i > 0 {
			name = args[0]
		}
		programArgs = args[i+1:]
	} else if len(args) > 0 {
		name = args[0]
	}

	s, _, err := mustSolution()
	if err != nil {
		return err
	}

	proj, err := resolveProject(s, name)
	if err != nil {
		return err
	}

	// 本次运行参数会持久化到项目配置中
	if len(programArgs) > 0 {
		cfg := proj.Config()
		cfg.RunArgs = strings.Join(programArgs, " ")
		if err := proj.SaveConfig(cfg); err != nil {
			return err
		}
	}

	exe := proj.ResolveExecutable()
	if exe == "" {
		return fmt.Errorf("no executable found for %s; run 'pm build' first", proj.Config().Name)
	}
	fmt.Printf("Running %s...\n", exe)

	return proj.Run()
}

// ===================== 依赖管理（项目级） =====================

// cmdDeps 实现 pm deps <install|add|remove|list> ...
func cmdDeps(args []string) error {
	if len(args) < 1 {
		printDepsUsage()
		return fmt.Errorf("missing deps subcommand")
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "install", "i":
		return depsInstall(rest)
	case "add":
		return depsAdd(rest)
	case "remove", "rm":
		return depsRemove(rest)
	case "list", "ls":
		return depsList(rest)
	default:
		printDepsUsage()
		return fmt.Errorf("unknown deps subcommand: %s", sub)
	}
}

func printDepsUsage() {
	fmt.Println("Usage:")
	fmt.Println("  pm deps install [project]              Install all dependencies of a project")
	fmt.Println("  pm deps add <pkg>[@ver] [-p project]   Add and install a dependency")
	fmt.Println("  pm deps remove <pkg> [-p project]      Remove a dependency (also its installed files)")
	fmt.Println("                     [--keep-files]      Only drop the declaration, keep files")
	fmt.Println("  pm deps list [project]                 List dependencies of a project")
}

// depsInstall 安装项目声明的全部依赖
func depsInstall(args []string) error {
	s, pm, err := mustSolution()
	if err != nil {
		return err
	}

	proj, err := resolveProject(s, firstArg(args))
	if err != nil {
		return err
	}

	cfg := proj.Config()
	if len(cfg.Dependencies) == 0 {
		fmt.Printf("%s has no declared dependencies.\n", cfg.Name)
		return nil
	}

	fmt.Printf("Installing %d dependency(ies) for %s...\n", len(cfg.Dependencies), cfg.Name)
	manifests, err := pm.InstallProjectDeps(proj)
	for _, m := range manifests {
		fmt.Printf("  + %s@%s\n", m.Name, m.Version)
	}
	if err != nil {
		return err
	}

	fmt.Println("Dependencies installed.")
	return nil
}

// depsAdd 添加并安装单个依赖
func depsAdd(args []string) error {
	fs := newFlagSet("deps add")
	projName := fs.String("p", "", "target project")
	fs.StringVar(projName, "project", "", "target project")

	flags, positional := normalizeArgs(args, map[string]bool{"-p": true, "--project": true})
	if err := fs.Parse(flags); err != nil {
		return err
	}

	if len(positional) < 1 {
		return fmt.Errorf("usage: pm deps add <pkg>[@ver] [-p project]")
	}
	dep := parsePkgSpec(positional[0])

	s, pm, err := mustSolution()
	if err != nil {
		return err
	}

	proj, err := resolveProject(s, *projName)
	if err != nil {
		return err
	}

	manifests, err := pm.InstallToProject(proj, dep)
	for _, m := range manifests {
		fmt.Printf("  + %s@%s\n", m.Name, m.Version)
	}
	if err != nil {
		return err
	}

	// 打印实际解析到的版本（未显式指定版本时取仓库中最新版）
	resolved := dep.Version
	if len(manifests) > 0 && manifests[0].Name == dep.Name {
		resolved = manifests[0].Version
	}
	if resolved == "" {
		resolved = "latest"
	}

	fmt.Printf("Added dependency %s@%s to %s\n", dep.Name, resolved, proj.Config().Name)
	return nil
}

// depsRemove 移除依赖声明，并清理安装时复制进项目的头文件与库文件
func depsRemove(args []string) error {
	fs := newFlagSet("deps remove")
	projName := fs.String("p", "", "target project")
	fs.StringVar(projName, "project", "", "target project")
	keepFiles := fs.Bool("keep-files", false, "only drop the declaration, keep installed files")
	fs.BoolVar(keepFiles, "k", false, "only drop the declaration, keep installed files")

	flags, positional := normalizeArgs(args, map[string]bool{"-p": true, "--project": true})
	if err := fs.Parse(flags); err != nil {
		return err
	}

	if len(positional) < 1 {
		return fmt.Errorf("usage: pm deps remove <pkg> [-p project] [--keep-files]")
	}
	name := positional[0]

	s, _, err := mustSolution()
	if err != nil {
		return err
	}

	proj, err := resolveProject(s, *projName)
	if err != nil {
		return err
	}

	cfg := proj.Config()
	found := false
	for _, d := range cfg.Dependencies {
		if d.Name == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("dependency %s not found in project %s", name, cfg.Name)
	}

	if *keepFiles {
		if err := proj.RemoveDependencyByName(name); err != nil {
			return err
		}
		fmt.Printf("Removed dependency %s from %s (installed files kept)\n", name, cfg.Name)
		return nil
	}

	removed, err := proj.UninstallDependency(name)
	if err != nil {
		return err
	}

	fmt.Printf("Removed dependency %s from %s\n", name, cfg.Name)
	for _, p := range removed {
		fmt.Printf("  - %s\n", p)
	}
	return nil
}

// depsList 列出项目依赖及其安装状态
func depsList(args []string) error {
	s, pm, err := mustSolution()
	if err != nil {
		return err
	}

	proj, err := resolveProject(s, firstArg(args))
	if err != nil {
		return err
	}

	cfg := proj.Config()
	if len(cfg.Dependencies) == 0 {
		fmt.Printf("%s has no dependencies.\n", cfg.Name)
		return nil
	}

	fmt.Printf("Dependencies of %s:\n", cfg.Name)
	for _, d := range cfg.Dependencies {
		fmt.Printf("  %-20s %-10s %s\n", d.Name, d.Version, proj.DependencyStatus(pm, d))
	}
	return nil
}

// ===================== 包仓库管理 =====================

// cmdPkg 实现 pm pkg <create|install|remove|list|search|dir> ...
func cmdPkg(args []string) error {
	if len(args) < 1 {
		printPkgUsage()
		return fmt.Errorf("missing pkg subcommand")
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "create", "build":
		return pkgCreate(rest)
	case "install", "i":
		return pkgInstall(rest)
	case "remove", "rm":
		return pkgRemove(rest)
	case "list", "ls":
		return pkgList(rest)
	case "search":
		return pkgSearch(rest)
	case "dir":
		return pkgDir(rest)
	default:
		printPkgUsage()
		return fmt.Errorf("unknown pkg subcommand: %s", sub)
	}
}

func printPkgUsage() {
	fmt.Println("Usage:")
	fmt.Println("  pm pkg create <dir> [-o output.pm] [-f] Build a .pm package from a directory")
	fmt.Println("  pm pkg install <pkg>[@ver] [-p proj]   Install a package into a project")
	fmt.Println("  pm pkg remove <pkg>[@ver]              Remove a package from the repository")
	fmt.Println("  pm pkg list                            List packages in the repository")
	fmt.Println("  pm pkg search [keyword]                Search packages in the repository")
	fmt.Println("  pm pkg dir                             Print the repository directory")
}

// pkgCreate 从目录打包为 .pm
func pkgCreate(args []string) error {
	fs := newFlagSet("pkg create")
	out := fs.String("o", "", "output .pm file path")
	fs.StringVar(out, "output", "", "output .pm file path")
	force := fs.Bool("f", false, "overwrite an existing package of the same name@version")
	fs.BoolVar(force, "force", false, "overwrite an existing package of the same name@version")

	flags, positional := normalizeArgs(args, map[string]bool{"-o": true, "--output": true})
	if err := fs.Parse(flags); err != nil {
		return err
	}

	if len(positional) < 1 {
		return fmt.Errorf("usage: pm pkg create <package-dir> [-o output.pm] [-f]")
	}

	_, pm, err := mustSolution()
	if err != nil {
		return err
	}

	file, err := pm.CreatePackageWithOptions(positional[0], core.CreateOptions{
		OutFile: *out,
		Force:   *force,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Package created: %s\n", file)
	if sum, err := core.FileSHA256(file); err == nil {
		fmt.Printf("  sha256: %s\n", sum)
	}
	return nil
}

// pkgInstall 把包安装到项目
func pkgInstall(args []string) error {
	fs := newFlagSet("pkg install")
	projName := fs.String("p", "", "target project")
	fs.StringVar(projName, "project", "", "target project")

	flags, positional := normalizeArgs(args, map[string]bool{"-p": true, "--project": true})
	if err := fs.Parse(flags); err != nil {
		return err
	}

	if len(positional) < 1 {
		return fmt.Errorf("usage: pm pkg install <pkg>[@ver] [-p project]")
	}
	dep := parsePkgSpec(positional[0])

	s, pm, err := mustSolution()
	if err != nil {
		return err
	}

	proj, err := resolveProject(s, *projName)
	if err != nil {
		return err
	}

	manifests, err := pm.InstallToProject(proj, dep)
	for _, m := range manifests {
		fmt.Printf("  + %s@%s\n", m.Name, m.Version)
	}
	if err != nil {
		return err
	}

	fmt.Printf("Installed %s into %s\n", dep.Name, proj.Config().Name)
	return nil
}

// pkgRemove 从仓库删除包
func pkgRemove(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: pm pkg remove <pkg>[@ver]")
	}
	dep := parsePkgSpec(args[0])

	_, pm, err := mustSolution()
	if err != nil {
		return err
	}

	if err := pm.RemovePackage(dep.Name, dep.Version); err != nil {
		return err
	}

	fmt.Printf("Removed package %s@%s from repository\n", dep.Name, dep.Version)
	return nil
}

// pkgList 列出仓库中的包
func pkgList(args []string) error {
	_, pm, err := mustSolution()
	if err != nil {
		return err
	}

	infos, err := pm.ListPackages()
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		fmt.Printf("Repository is empty: %s\n", pm.RepoDir())
		return nil
	}

	for _, dir := range pm.RepoDirs() {
		fmt.Printf("Repository: %s\n", dir)
	}
	for _, info := range infos {
		desc := info.Manifest.Description
		if desc == "" {
			desc = "-"
		}
		tag := ""
		if pm.IsBuiltin(info.File) {
			tag = "  [builtin]"
		}
		fmt.Printf("  %-20s %-10s %8d B  %s%s\n",
			info.Manifest.Name, info.Manifest.Version, info.Size, desc, tag)
	}
	return nil
}

// pkgSearch 搜索仓库中的包
func pkgSearch(args []string) error {
	_, pm, err := mustSolution()
	if err != nil {
		return err
	}

	keyword := strings.Join(args, " ")
	infos, err := pm.SearchPackages(keyword)
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		fmt.Printf("No packages matched %q\n", keyword)
		return nil
	}

	for _, info := range infos {
		desc := info.Manifest.Description
		if desc == "" {
			desc = "-"
		}
		fmt.Printf("  %-20s %-10s %s\n", info.Manifest.Name, info.Manifest.Version, desc)
	}
	return nil
}

// pkgDir 打印仓库目录
func pkgDir(args []string) error {
	_, pm, err := mustSolution()
	if err != nil {
		return err
	}
	fmt.Println(pm.RepoDir())
	return nil
}
