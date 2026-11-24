package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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
	case "update", "upgrade", "up":
		return depsUpdate(rest)
	case "rollback", "undo":
		return depsRollback(rest)
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
	fmt.Println("  pm deps update [project]               Update all dependencies to the latest versions")
	fmt.Println("                     [--dry-run] [--yes]  Dry-run by default; --yes applies")
	fmt.Println("                     [--pre]              Include pre-release versions")
	fmt.Println("  pm deps rollback [project]             Restore versions from the last update snapshot")
	fmt.Println("                     [--dry-run] [--yes] [--clean]")
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

// dependencyUpdate 描述单个依赖的更新计划。
type dependencyUpdate struct {
	Name   string
	From   string
	To     string
	Action string // update / up-to-date / missing / newer
}

// depsUpdate 把项目声明的全部依赖更新到仓库中的最新版本。
// 默认只打印计划；--yes 才执行。
func depsUpdate(args []string) error {
	fs := newFlagSet("deps update")
	projName := fs.String("p", "", "target project")
	fs.StringVar(projName, "project", "", "target project")
	yes := fs.Bool("yes", false, "apply the updates")
	fs.BoolVar(yes, "y", false, "apply the updates")
	dryRun := fs.Bool("dry-run", false, "only print the plan")
	pre := fs.Bool("pre", false, "include pre-release versions")

	flags, positional := normalizeArgs(args, map[string]bool{"-p": true, "--project": true})
	if err := fs.Parse(flags); err != nil {
		return err
	}
	if *yes && *dryRun {
		return fmt.Errorf("--yes and --dry-run are mutually exclusive")
	}

	s, pm, err := mustSolution()
	if err != nil {
		return err
	}

	name := *projName
	if strings.TrimSpace(name) == "" {
		name = firstArg(positional)
	}
	proj, err := resolveProject(s, name)
	if err != nil {
		return err
	}

	plans, err := planDependencyUpdates(pm, proj, *pre)
	if err != nil {
		return err
	}
	if len(plans) == 0 {
		fmt.Printf("%s has no declared dependencies.\n", proj.Config().Name)
		return nil
	}

	printDependencyUpdates(proj, plans)
	updates := countDependencyUpdates(plans, "update")
	if *dryRun || !*yes {
		if updates == 0 {
			fmt.Println("\nNo dependencies can be updated.")
			return nil
		}
		fmt.Printf("\n%d update(s) available. Run \"pm deps update --yes\" to apply.\n", updates)
		return nil
	}
	if updates == 0 {
		fmt.Println("\nNo dependencies can be updated.")
		return nil
	}

	// 先保存回退快照，任何一步失败都可以用 pm deps rollback 恢复。
	if err := proj.SaveDependencyBackup("deps update"); err != nil {
		return fmt.Errorf("save rollback snapshot: %w", err)
	}

	fmt.Printf("\nUpdating %d dependency(ies) for %s...\n", updates, proj.Config().Name)

	applied := 0
	var failed []string
	for _, plan := range plans {
		if plan.Action != "update" {
			continue
		}

		// 前面的更新可能已通过递归依赖把这个包带到了更高版本，重新检查一次。
		current := dependencyVersion(proj.Config(), plan.Name)
		if current != "" && core.CompareVersions(plan.To, current) <= 0 {
			fmt.Printf("  = %s@%s (already at or newer than %s)\n", plan.Name, current, plan.To)
			continue
		}

		// 按计划中的目标版本精确安装，避免默认更新时又选中预发布版。
		manifests, ierr := pm.InstallToProject(proj, core.Dependency{Name: plan.Name, Version: plan.To})
		if ierr != nil {
			fmt.Printf("  ! %s: %v\n", plan.Name, ierr)
			failed = append(failed, plan.Name)
			continue
		}
		for _, m := range manifests {
			fmt.Printf("  + %s@%s\n", m.Name, m.Version)
		}
		applied++
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to update %d dependency(ies): %s", len(failed), strings.Join(failed, ", "))
	}
	if applied == 0 {
		fmt.Println("All dependencies are already up to date.")
		return nil
	}
	fmt.Printf("Updated %d dependency(ies).\n", applied)
	return nil
}

// latestUpdateTarget 返回更新目标版本：默认取最高正式版；
// 没有正式版时退回最高预发布版；includePre 为真时直接取最高版本。
func latestUpdateTarget(pm *core.PackageManager, name string, includePre bool) (core.PackageInfo, error) {
	if includePre {
		return pm.LatestPackage(name)
	}
	stable, err := pm.LatestStablePackage(name)
	if err == nil {
		return stable, nil
	}
	return pm.LatestPackage(name)
}

// planDependencyUpdates 计算项目各依赖的更新计划。
// 默认只考虑正式版；includePre 为真时也考虑预发布版。
func planDependencyUpdates(pm *core.PackageManager, proj *core.Project, includePre bool) ([]dependencyUpdate, error) {
	cfg := proj.Config()
	seen := make(map[string]bool, len(cfg.Dependencies))
	plans := make([]dependencyUpdate, 0, len(cfg.Dependencies))

	for _, d := range cfg.Dependencies {
		name := strings.TrimSpace(d.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true

		latest, err := latestUpdateTarget(pm, name, includePre)
		if err != nil {
			plans = append(plans, dependencyUpdate{
				Name:   name,
				From:   d.Version,
				Action: "missing",
			})
			continue
		}

		plan := dependencyUpdate{
			Name: name,
			From: d.Version,
			To:   latest.Manifest.Version,
		}
		switch {
		case d.Version == "":
			plan.Action = "update"
		default:
			switch core.CompareVersions(latest.Manifest.Version, d.Version) {
			case 1:
				plan.Action = "update"
			case 0:
				plan.Action = "up-to-date"
			default:
				plan.Action = "newer"
			}
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

// printDependencyUpdates 打印更新计划。
func printDependencyUpdates(proj *core.Project, plans []dependencyUpdate) {
	fmt.Printf("Dependencies of %s:\n", proj.Config().Name)
	for _, p := range plans {
		switch p.Action {
		case "update":
			fmt.Printf("  ~ %-20s %-12s -> %s\n", p.Name, versionLabel(p.From), p.To)
		case "up-to-date":
			fmt.Printf("  = %-20s %-12s (up to date)\n", p.Name, versionLabel(p.From))
		case "missing":
			fmt.Printf("  ! %-20s %-12s (not found in repository)\n", p.Name, versionLabel(p.From))
		case "newer":
			fmt.Printf("  > %-20s %-12s (newer than update target %s; skipped)\n",
				p.Name, versionLabel(p.From), p.To)
		}
	}
}

func countDependencyUpdates(plans []dependencyUpdate, action string) int {
	n := 0
	for _, p := range plans {
		if p.Action == action {
			n++
		}
	}
	return n
}

func dependencyVersion(cfg core.ProjectConfig, name string) string {
	for _, d := range cfg.Dependencies {
		if d.Name == name {
			return d.Version
		}
	}
	return ""
}

func versionLabel(v string) string {
	if strings.TrimSpace(v) == "" {
		return "(none)"
	}
	return v
}

// dependencyRollback 单个依赖的回退计划。
type dependencyRollback struct {
	Name    string
	Current string
	Target  string
	Action  string // restore / up-to-date / missing / extra
}

// depsRollback 恢复到上一次 pm deps update 之前的依赖版本。
// 默认只打印计划；--yes 才执行。快照中的旧版本必须仍在仓库中。
func depsRollback(args []string) error {
	fs := newFlagSet("deps rollback")
	projName := fs.String("p", "", "target project")
	fs.StringVar(projName, "project", "", "target project")
	yes := fs.Bool("yes", false, "apply the rollback")
	fs.BoolVar(yes, "y", false, "apply the rollback")
	dryRun := fs.Bool("dry-run", false, "only print the plan")
	clean := fs.Bool("clean", false, "remove dependencies that are not in the snapshot")

	flags, positional := normalizeArgs(args, map[string]bool{"-p": true, "--project": true})
	if err := fs.Parse(flags); err != nil {
		return err
	}
	if *yes && *dryRun {
		return fmt.Errorf("--yes and --dry-run are mutually exclusive")
	}

	s, pm, err := mustSolution()
	if err != nil {
		return err
	}

	name := *projName
	if strings.TrimSpace(name) == "" {
		name = firstArg(positional)
	}
	proj, err := resolveProject(s, name)
	if err != nil {
		return err
	}

	backup, err := proj.LoadDependencyBackup()
	if err != nil {
		return fmt.Errorf("no rollback snapshot for project %s: %w", proj.Config().Name, err)
	}

	plans := planDependencyRollback(pm, proj, backup)
	printDependencyRollback(backup, plans)

	restores := countRollback(plans, "restore")
	extras := countRollback(plans, "extra")
	missing := countRollback(plans, "missing")

	if *dryRun || !*yes {
		if restores == 0 && (!*clean || extras == 0) {
			fmt.Println("\nAlready at the snapshot versions.")
			return nil
		}
		fmt.Printf("\n%d restore(s) available.\n", restores)
		if *clean && extras > 0 {
			fmt.Printf("%d dependency(ies) not in the snapshot will be removed.\n", extras)
		}
		fmt.Println(`Run "pm deps rollback --yes" to apply.`)
		return nil
	}

	applied := 0
	var failed []string

	if *clean {
		for _, p := range plans {
			if p.Action != "extra" {
				continue
			}
			removed, rerr := proj.UninstallDependency(p.Name)
			if rerr != nil {
				fmt.Printf("  ! %s: %v\n", p.Name, rerr)
				failed = append(failed, p.Name)
				continue
			}
			fmt.Printf("  - %s@%s\n", p.Name, versionLabel(p.Current))
			for _, path := range removed {
				fmt.Printf("    - %s\n", path)
			}
			applied++
		}
	}

	for _, p := range plans {
		if p.Action != "restore" {
			continue
		}

		// 前面的恢复可能已经把这个包带到目标版本。
		current := dependencyVersion(proj.Config(), p.Name)
		status := proj.DependencyStatus(pm, core.Dependency{Name: p.Name, Version: p.Target})
		if current == p.Target && status == "installed" {
			fmt.Printf("  = %s@%s (already restored)\n", p.Name, p.Target)
			continue
		}

		manifests, ierr := pm.InstallToProject(proj, core.Dependency{Name: p.Name, Version: p.Target})
		if ierr != nil {
			fmt.Printf("  ! %s: %v\n", p.Name, ierr)
			failed = append(failed, p.Name)
			continue
		}
		for _, m := range manifests {
			fmt.Printf("  + %s@%s\n", m.Name, m.Version)
		}
		applied++
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to roll back %d dependency(ies): %s", len(failed), strings.Join(failed, ", "))
	}
	if missing > 0 {
		return fmt.Errorf("rollback incomplete: %d snapshot version(s) not found in repository", missing)
	}
	if applied == 0 {
		fmt.Println("Already at the snapshot versions.")
		return nil
	}
	fmt.Printf("Rolled back %d dependency(ies) to the previous snapshot.\n", applied)
	return nil
}

// planDependencyRollback 计算回退计划。
func planDependencyRollback(pm *core.PackageManager, proj *core.Project, backup *core.DependencyBackup) []dependencyRollback {
	cfg := proj.Config()
	current := make(map[string]string, len(cfg.Dependencies))
	for _, d := range cfg.Dependencies {
		current[d.Name] = d.Version
	}

	inBackup := make(map[string]bool, len(backup.Dependencies))
	plans := make([]dependencyRollback, 0, len(backup.Dependencies)+len(cfg.Dependencies))

	for _, d := range backup.Dependencies {
		name := strings.TrimSpace(d.Name)
		if name == "" || inBackup[name] {
			continue
		}
		inBackup[name] = true

		p := dependencyRollback{
			Name:    name,
			Current: current[name],
			Target:  d.Version,
		}
		switch {
		case strings.TrimSpace(d.Version) == "":
			p.Action = "missing"
		case pm.ManifestOf(name, d.Version) == nil:
			p.Action = "missing"
		case current[name] == d.Version &&
			proj.DependencyStatus(pm, core.Dependency{Name: name, Version: d.Version}) == "installed":
			p.Action = "up-to-date"
		default:
			p.Action = "restore"
		}
		plans = append(plans, p)
	}

	for _, d := range cfg.Dependencies {
		name := strings.TrimSpace(d.Name)
		if name == "" || inBackup[name] {
			continue
		}
		plans = append(plans, dependencyRollback{
			Name:    name,
			Current: d.Version,
			Action:  "extra",
		})
	}
	return plans
}

// printDependencyRollback 打印回退计划。
func printDependencyRollback(backup *core.DependencyBackup, plans []dependencyRollback) {
	fmt.Printf("Snapshot: %s", backup.CreatedAt.Local().Format("2006-01-02 15:04:05"))
	if backup.Reason != "" {
		fmt.Printf(" (%s)", backup.Reason)
	}
	fmt.Println()

	for _, p := range plans {
		switch p.Action {
		case "restore":
			fmt.Printf("  ~ %-20s %-12s -> %s\n", p.Name, versionLabel(p.Current), p.Target)
		case "up-to-date":
			fmt.Printf("  = %-20s %-12s\n", p.Name, versionLabel(p.Current))
		case "missing":
			fmt.Printf("  ! %-20s %-12s (snapshot version %s not found in repository)\n",
				p.Name, versionLabel(p.Current), versionLabel(p.Target))
		case "extra":
			fmt.Printf("  + %-20s %-12s (not in snapshot; kept unless --clean)\n",
				p.Name, versionLabel(p.Current))
		}
	}
}

func countRollback(plans []dependencyRollback, action string) int {
	n := 0
	for _, p := range plans {
		if p.Action == action {
			n++
		}
	}
	return n
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
	case "prune":
		return pkgPrune(rest)
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
	fmt.Println("  pm pkg list [-a] [name...]             List packages (one row per package; -a shows all versions)")
	fmt.Println("  pm pkg search [-a] [keyword]           Search packages (grouped by name unless -a)")
	fmt.Println("  pm pkg prune [name...]                 Remove old versions from the repository")
	fmt.Println("              [--keep N] [--keep-major N] [--older-than DUR]")
	fmt.Println("              [--yes] [--dry-run] [--force]")
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

// pkgList 列出仓库中的包。
// 默认每个包一行（显示最新版本与版本数）；-a 显示全部版本；带包名时显示该包的版本历史。
func pkgList(args []string) error {
	fs := newFlagSet("pkg list")
	all := fs.Bool("a", false, "show every version")
	fs.BoolVar(all, "all", false, "show every version")

	flags, positional := normalizeArgs(args, nil)
	if err := fs.Parse(flags); err != nil {
		return err
	}
	names := cleanNames(positional)

	s, pm, err := mustSolution()
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

	if *all {
		filtered, missing := filterPackageInfos(infos, names)
		if len(missing) > 0 {
			return fmt.Errorf("package(s) not found in repository: %s", strings.Join(missing, ", "))
		}
		printPackageRows(pm, filtered)
		return nil
	}
	fmt.Println()

	if len(names) > 0 {
		byName := make(map[string][]core.PackageInfo, len(names))
		for _, info := range infos {
			byName[info.Manifest.Name] = append(byName[info.Manifest.Name], info)
		}
		refs := collectProjectRefs(s)
		for _, name := range names {
			versions := byName[name]
			if len(versions) == 0 {
				return fmt.Errorf("package %s not found in repository (%s)", name, pm.RepoDir())
			}
			printPackageVersions(pm, versions, refs[name])
		}
		return nil
	}

	printPackageSummaries(pm, core.SummarizePackages(infos), `Use "pm pkg list -a" for all versions; "pm pkg list <name>" for one package's history.`)
	return nil
}

// pkgSearch 搜索仓库中的包。
// 默认按包名折叠（显示匹配到的版本数）；-a 显示每个匹配版本。
func pkgSearch(args []string) error {
	fs := newFlagSet("pkg search")
	all := fs.Bool("a", false, "show every matching version")
	fs.BoolVar(all, "all", false, "show every matching version")

	flags, positional := normalizeArgs(args, nil)
	if err := fs.Parse(flags); err != nil {
		return err
	}

	_, pm, err := mustSolution()
	if err != nil {
		return err
	}

	keyword := strings.Join(positional, " ")
	infos, err := pm.SearchPackages(keyword)
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		fmt.Printf("No packages matched %q\n", keyword)
		return nil
	}

	if *all {
		printPackageRows(pm, infos)
		return nil
	}

	printPackageSummaries(pm, core.SummarizePackages(infos), `Use "pm pkg search -a <keyword>" to show every matching version.`)
	return nil
}

// pkgPrune 按策略清理用户仓库中的旧版本（默认只打印计划）。
func pkgPrune(args []string) error {
	fs := newFlagSet("pkg prune")
	keep := fs.Int("keep", 2, "keep the newest N versions per package")
	keepMajor := fs.Int("keep-major", 0, "keep the newest N versions per major version (0 disables)")
	olderThan := fs.String("older-than", "", "only remove versions older than this (e.g. 30d, 720h)")
	yes := fs.Bool("yes", false, "delete the selected versions (default is dry-run)")
	dryRun := fs.Bool("dry-run", false, "only print the plan")
	force := fs.Bool("force", false, "ignore versions referenced by registered projects")

	flags, positional := normalizeArgs(args, map[string]bool{
		"-keep": true, "--keep": true,
		"-keep-major": true, "--keep-major": true,
		"-older-than": true, "--older-than": true,
	})
	if err := fs.Parse(flags); err != nil {
		return err
	}
	if *keep < 1 {
		return fmt.Errorf("--keep must be >= 1")
	}
	if *keepMajor < 0 {
		return fmt.Errorf("--keep-major must be >= 0")
	}
	if *yes && *dryRun {
		return fmt.Errorf("--yes and --dry-run are mutually exclusive")
	}
	age, err := parseAge(*olderThan)
	if err != nil {
		return err
	}

	s, pm, err := mustSolution()
	if err != nil {
		return err
	}

	result, err := pm.Prune(core.PruneOptions{
		Keep:      *keep,
		KeepMajor: *keepMajor,
		OlderThan: age,
		Names:     cleanNames(positional),
		Force:     *force,
		DryRun:    !*yes,
		Protected: protectedVersions(s),
	})
	if err != nil {
		return err
	}

	printPruneResult(pm, result, *keep, *keepMajor, age, !*yes)
	return nil
}

// ===================== 包列表输出辅助 =====================

// cleanNames 去掉空白与重复的包名，保持原有顺序。
func cleanNames(names []string) []string {
	seen := make(map[string]bool, len(names))
	result := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		result = append(result, n)
	}
	return result
}

// filterPackageInfos 按包名过滤，并返回未找到的包名。
func filterPackageInfos(infos []core.PackageInfo, names []string) ([]core.PackageInfo, []string) {
	if len(names) == 0 {
		return infos, nil
	}
	wanted := make(map[string]bool, len(names))
	for _, n := range names {
		wanted[n] = true
	}

	found := make(map[string]bool, len(names))
	filtered := make([]core.PackageInfo, 0, len(infos))
	for _, info := range infos {
		if wanted[info.Manifest.Name] {
			filtered = append(filtered, info)
			found[info.Manifest.Name] = true
		}
	}

	var missing []string
	for _, n := range names {
		if !found[n] {
			missing = append(missing, n)
		}
	}
	return filtered, missing
}

// printPackageRows 打印扁平的全量版本列表（旧格式，供 -a 使用）。
func printPackageRows(pm *core.PackageManager, infos []core.PackageInfo) {
	for _, info := range infos {
		tag := ""
		if pm.IsBuiltin(info.File) {
			tag = "  [builtin]"
		}
		fmt.Printf("  %-20s %-10s %8d B  %s%s\n",
			info.Manifest.Name, info.Manifest.Version, info.Size, descriptionOrDash(info), tag)
	}
}

// printPackageSummaries 打印按包名折叠的列表。
func printPackageSummaries(pm *core.PackageManager, summaries []core.PackageSummary, hint string) {
	fmt.Printf("  %-20s %-10s %4s  %-9s %s\n", "NAME", "LATEST", "VERS", "SIZE", "DESCRIPTION")

	totalVersions := 0
	var totalSize int64
	for _, s := range summaries {
		tag := ""
		if pm.IsBuiltin(s.Latest.File) {
			tag = "  [builtin]"
		}
		fmt.Printf("  %-20s %-10s %4d  %-9s %s%s\n",
			s.Name, s.Latest.Manifest.Version, len(s.Versions),
			core.FormatSize(s.TotalSize), descriptionOrDash(s.Latest), tag)
		totalVersions += len(s.Versions)
		totalSize += s.TotalSize
	}

	fmt.Printf("\n  %d packages, %d versions, %s\n", len(summaries), totalVersions, core.FormatSize(totalSize))
	if hint != "" {
		fmt.Printf("  %s\n", hint)
	}
}

// printPackageVersions 打印单个包的版本历史。
func printPackageVersions(pm *core.PackageManager, versions []core.PackageInfo, refs map[string][]string) {
	name := versions[0].Manifest.Name
	var total int64
	for _, v := range versions {
		total += v.Size
	}

	fmt.Printf("Package: %s  (%d versions, %s)\n\n", name, len(versions), core.FormatSize(total))
	fmt.Printf("  %-10s %-10s %-8s %-16s %s\n", "VERSION", "SIZE", "SOURCE", "USED BY", "FILE")
	for _, v := range versions {
		source := "user"
		if pm.IsBuiltin(v.File) {
			source = "builtin"
		}
		fmt.Printf("  %-10s %-10s %-8s %-16s %s\n",
			v.Manifest.Version, core.FormatSize(v.Size), source,
			usedByText(refs, v.Manifest.Version), v.File)
	}
	fmt.Println()
}

// usedByText 把某个版本的项目引用拼成一行文本。
func usedByText(refs map[string][]string, version string) string {
	names := make([]string, 0, len(refs[version])+len(refs["*"]))
	names = append(names, refs[version]...)
	names = append(names, refs["*"]...)
	if len(names) == 0 {
		return "-"
	}
	sort.Strings(names)

	// 去重（"*" 与具体版本可能引用同一项目）
	unique := names[:0]
	for _, n := range names {
		if len(unique) == 0 || unique[len(unique)-1] != n {
			unique = append(unique, n)
		}
	}
	return strings.Join(unique, ", ")
}

// descriptionOrDash 返回包描述，空描述显示为 "-"。
func descriptionOrDash(info core.PackageInfo) string {
	if strings.TrimSpace(info.Manifest.Description) == "" {
		return "-"
	}
	return info.Manifest.Description
}

// collectProjectRefs 收集已注册项目对包版本的引用：包名 -> 版本 -> 项目名。
// 依赖未写版本号时用 "*" 表示该包全部版本。
func collectProjectRefs(s *core.Solution) map[string]map[string][]string {
	refs := make(map[string]map[string][]string)
	if s == nil {
		return refs
	}

	for _, path := range s.ProjectPaths() {
		proj := s.GetProject(path)
		if proj == nil {
			continue
		}
		cfg := proj.Config()
		projName := strings.TrimSpace(cfg.Name)
		if projName == "" {
			projName = filepath.Base(path)
		}
		for _, d := range cfg.Dependencies {
			name := strings.TrimSpace(d.Name)
			if name == "" {
				continue
			}
			ver := strings.TrimSpace(d.Version)
			if ver == "" {
				ver = "*"
			}
			if refs[name] == nil {
				refs[name] = make(map[string][]string)
			}
			if !containsString(refs[name][ver], projName) {
				refs[name][ver] = append(refs[name][ver], projName)
			}
		}

		// 回退快照里的旧版本也视为仍在使用，避免 prune 把可回退的包删掉。
		if backup, berr := proj.LoadDependencyBackup(); berr == nil {
			label := projName + " (rollback)"
			for _, d := range backup.Dependencies {
				name := strings.TrimSpace(d.Name)
				if name == "" {
					continue
				}
				ver := strings.TrimSpace(d.Version)
				if ver == "" {
					ver = "*"
				}
				if refs[name] == nil {
					refs[name] = make(map[string][]string)
				}
				if !containsString(refs[name][ver], label) {
					refs[name][ver] = append(refs[name][ver], label)
				}
			}
		}
	}

	for name := range refs {
		for ver := range refs[name] {
			sort.Strings(refs[name][ver])
		}
	}
	return refs
}

// protectedVersions 把项目引用转换成 Prune 需要的保护集合。
func protectedVersions(s *core.Solution) map[string]map[string]bool {
	refs := collectProjectRefs(s)
	out := make(map[string]map[string]bool, len(refs))
	for name, versions := range refs {
		out[name] = make(map[string]bool, len(versions))
		for ver := range versions {
			out[name][ver] = true
		}
	}
	return out
}

func containsString(list []string, item string) bool {
	for _, e := range list {
		if e == item {
			return true
		}
	}
	return false
}

// parseAge 解析清理时长，支持 Go duration（12h、30m）以及 d/w 后缀（30d、2w）。
func parseAge(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	mult := time.Duration(0)
	value := s
	switch {
	case strings.HasSuffix(s, "d"):
		mult = 24 * time.Hour
		value = strings.TrimSuffix(s, "d")
	case strings.HasSuffix(s, "w"):
		mult = 7 * 24 * time.Hour
		value = strings.TrimSuffix(s, "w")
	}
	if mult > 0 {
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(n) * mult, nil
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q (use e.g. 30d, 12h)", s)
	}
	return d, nil
}

// printPruneResult 打印清理计划或执行结果。
func printPruneResult(pm *core.PackageManager, result core.PruneResult, keep, keepMajor int, age time.Duration, dryRun bool) {
	fmt.Printf("Repository: %s\n", pm.RepoDir())
	policy := fmt.Sprintf("Policy: keep %d latest version(s) per package", keep)
	if keepMajor > 0 {
		policy += fmt.Sprintf("; keep %d latest per major", keepMajor)
	}
	if age > 0 {
		policy += fmt.Sprintf("; only versions older than %s", age)
	}
	fmt.Println(policy)
	fmt.Println()

	removeCount := 0
	var removeSize int64
	for _, d := range result.Decisions {
		switch d.Action {
		case "remove":
			removeCount++
			removeSize += d.Info.Size
			fmt.Printf("  REMOVE  %s@%s  %s  (%s)\n",
				d.Info.Manifest.Name, d.Info.Manifest.Version,
				core.FormatSize(d.Info.Size), pruneReasonText(d.Reason))
		case "keep":
			switch d.Reason {
			case "builtin", "used-by-project", "latest-stable", "newer-than":
				fmt.Printf("  KEEP    %s@%s  %s  (%s)\n",
					d.Info.Manifest.Name, d.Info.Manifest.Version,
					core.FormatSize(d.Info.Size), pruneReasonText(d.Reason))
			}
		}
	}

	if len(result.OrphanSidecars) > 0 {
		if dryRun {
			fmt.Printf("\nWould remove %d orphan checksum file(s):\n", len(result.OrphanSidecars))
		} else {
			fmt.Printf("\nRemoved %d orphan checksum file(s):\n", len(result.OrphanSidecars))
		}
		for _, f := range result.OrphanSidecars {
			fmt.Printf("  %s\n", f)
		}
	}

	if removeCount == 0 && len(result.OrphanSidecars) == 0 {
		fmt.Println("Nothing to prune.")
		return
	}
	if removeCount == 0 {
		return
	}

	if dryRun {
		fmt.Printf("\nWould remove %d of %d versions, free %s.\n",
			removeCount, len(result.Decisions), core.FormatSize(removeSize))
		fmt.Println(`Run "pm pkg prune --yes" to delete.`)
		return
	}
	fmt.Printf("\nRemoved %d of %d versions, freed %s.\n",
		len(result.Removed), len(result.Decisions), core.FormatSize(result.Freed))
}

// pruneReasonText 把清理原因转成可读文本。
func pruneReasonText(reason string) string {
	switch reason {
	case "outside-retention":
		return "outside retention"
	case "used-by-project":
		return "used by project"
	case "latest-stable":
		return "latest stable"
	case "keep-major":
		return "keep-major policy"
	case "keep-policy":
		return "keep policy"
	case "newer-than":
		return "newer than cutoff"
	case "builtin":
		return "builtin (read-only)"
	default:
		return reason
	}
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
