package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rjf12138/project_manager/core"
)

// newStatusProject 创建一个隔离的临时项目（HOME 指向临时目录）
func newStatusProject(t *testing.T) *core.Project {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	full, err := core.CreateProject("Status", t.TempDir())
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	proj, err := core.LoadProject(full)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	return proj
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

// 依赖状态必须按该依赖自己的文件判断。
// 回归：旧实现只看 lib/debug 目录是否非空，只要项目里装了任意一个库，
// 所有未安装的依赖都会被误报成 libs-only。
func TestDependencyStatus(t *testing.T) {
	proj := newStatusProject(t)
	pm := core.NewPackageManager()
	if err := pm.Init(); err != nil {
		t.Fatal(err)
	}
	dirs := proj.DirInfo()

	if err := os.MkdirAll(filepath.Join(dirs.ExternInc, "alpha"), 0755); err != nil { // 头 + 库
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dirs.LibDebug, "libalpha.a"), "x")

	if err := os.MkdirAll(filepath.Join(dirs.ExternInc, "beta"), 0755); err != nil { // 只有头
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(dirs.LibDebug, "libgamma.a"), "x") // 只有库

	// delta 什么都没有

	cases := []struct{ name, want string }{
		{"alpha", "installed"},
		{"beta", "headers-only"},
		{"gamma", "libs-only"},
		{"delta", "missing"},
	}
	for _, c := range cases {
		got := proj.DependencyStatus(pm, core.Dependency{Name: c.name, Version: "1.0.0"})
		if got != c.want {
			t.Errorf("dependencyStatus(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

// link_libs 与包名不同时，状态要按 link_libs 找库文件，而不是按包名猜
func TestDependencyStatusUsesLinkLibs(t *testing.T) {
	proj := newStatusProject(t)
	dirs := proj.DirInfo()

	pm := core.NewPackageManager()
	if err := pm.Init(); err != nil {
		t.Fatal(err)
	}

	// 造一个「包名 pkgname、link_libs 为 reallib」的包并入库
	src := t.TempDir()
	writeTestFile(t, filepath.Join(src, "manifest.json"),
		`{"name":"pkgname","version":"1.0.0","link_libs":["reallib"]}`)
	writeTestFile(t, filepath.Join(src, "include", "pkgname.h"), "// p\n")
	if _, err := pm.CreatePackage(src, ""); err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(dirs.ExternInc, "pkgname"), 0755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dirs.LibDebug, "libreallib.a"), "x")

	dep := core.Dependency{Name: "pkgname", Version: "1.0.0"}
	if got := proj.DependencyStatus(pm, dep); got != "installed" {
		t.Errorf("got %q, want installed (should locate libreallib.a via link_libs)", got)
	}

	// 把库换成与包名同名但并非声明的 link_lib：应判为只有头文件
	if err := os.Remove(filepath.Join(dirs.LibDebug, "libreallib.a")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dirs.LibDebug, "libpkgname.a"), "x")
	if got := proj.DependencyStatus(pm, dep); got != "headers-only" {
		t.Errorf("got %q, want headers-only (libpkgname.a is not the declared link lib)", got)
	}
}

func TestParseAge(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"", 0, false},
		{"12h", 12 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"30d", 30 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"bad", 0, true},
		{"-1d", 0, true},
		{"1.5d", 0, true},
	}
	for _, c := range cases {
		got, err := parseAge(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("parseAge(%q) error = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("parseAge(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestFilterPackageInfos(t *testing.T) {
	infos := []core.PackageInfo{
		{Manifest: core.PackageManifest{Name: "alpha", Version: "1.0.0"}},
		{Manifest: core.PackageManifest{Name: "alpha", Version: "1.1.0"}},
		{Manifest: core.PackageManifest{Name: "beta", Version: "1.0.0"}},
	}

	filtered, missing := filterPackageInfos(infos, []string{"alpha", "missing"})
	if len(filtered) != 2 {
		t.Errorf("filtered = %d, want 2", len(filtered))
	}
	if len(missing) != 1 || missing[0] != "missing" {
		t.Errorf("missing = %v, want [missing]", missing)
	}

	all, missing := filterPackageInfos(infos, nil)
	if len(all) != len(infos) || len(missing) != 0 {
		t.Errorf("no-filter result = %d/%v, want all/none", len(all), missing)
	}
}

func TestUsedByText(t *testing.T) {
	refs := map[string][]string{
		"1.0.0": {"demo"},
		"*":     {"app", "demo"},
	}
	if got := usedByText(refs, "1.0.0"); got != "app, demo" {
		t.Errorf("usedByText = %q, want %q", got, "app, demo")
	}
	if got := usedByText(refs, "2.0.0"); got != "app, demo" {
		t.Errorf("usedByText(*) = %q, want %q", got, "app, demo")
	}
	if got := usedByText(nil, "1.0.0"); got != "-" {
		t.Errorf("usedByText(nil) = %q, want -", got)
	}
}

func TestPlanDependencyUpdates(t *testing.T) {
	proj := newStatusProject(t)
	pm := core.NewPackageManager()
	if err := pm.Init(); err != nil {
		t.Fatalf("pm.Init: %v", err)
	}

	create := func(version string) {
		src := t.TempDir()
		writeTestFile(t, filepath.Join(src, "manifest.json"),
			`{"name":"mylib","version":"`+version+`","link_libs":["mylib"]}`)
		writeTestFile(t, filepath.Join(src, "include", "mylib.h"), "// mylib\n")
		writeTestFile(t, filepath.Join(src, "lib", "debug", "libmylib.a"), "debug")
		writeTestFile(t, filepath.Join(src, "lib", "release", "libmylib.a"), "release")
		if _, err := pm.CreatePackage(src, ""); err != nil {
			t.Fatalf("CreatePackage(%s): %v", version, err)
		}
	}
	create("1.0.0")
	create("1.1.0")
	create("1.2.0-rc1")

	if _, err := pm.InstallToProject(proj, core.Dependency{Name: "mylib", Version: "1.0.0"}); err != nil {
		t.Fatalf("InstallToProject: %v", err)
	}

	// 默认只更新到最高正式版
	plans, err := planDependencyUpdates(pm, proj, false)
	if err != nil {
		t.Fatalf("planDependencyUpdates: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("plans = %+v, want 1 entry", plans)
	}
	if p := plans[0]; p.Action != "update" || p.From != "1.0.0" || p.To != "1.1.0" {
		t.Errorf("plan = %+v, want update 1.0.0 -> 1.1.0", p)
	}

	// --pre 时允许更新到预发布版
	plans, err = planDependencyUpdates(pm, proj, true)
	if err != nil {
		t.Fatalf("planDependencyUpdates(--pre): %v", err)
	}
	if len(plans) != 1 || plans[0].To != "1.2.0-rc1" {
		t.Errorf("plans = %+v, want target 1.2.0-rc1", plans)
	}
}

func TestPlanDependencyRollback(t *testing.T) {
	proj := newStatusProject(t)
	pm := core.NewPackageManager()
	if err := pm.Init(); err != nil {
		t.Fatalf("pm.Init: %v", err)
	}

	create := func(name, version string) {
		src := t.TempDir()
		writeTestFile(t, filepath.Join(src, "manifest.json"),
			`{"name":"`+name+`","version":"`+version+`","link_libs":["`+name+`"]}`)
		writeTestFile(t, filepath.Join(src, "include", name+".h"), "//\n")
		if _, err := pm.CreatePackage(src, ""); err != nil {
			t.Fatalf("CreatePackage(%s@%s): %v", name, version, err)
		}
	}
	create("mylib", "1.0.0")
	create("mylib", "1.1.0")
	create("other", "1.0.0")

	if _, err := pm.InstallToProject(proj, core.Dependency{Name: "mylib", Version: "1.0.0"}); err != nil {
		t.Fatalf("InstallToProject(mylib@1.0.0): %v", err)
	}
	if err := proj.SaveDependencyBackup("test"); err != nil {
		t.Fatalf("SaveDependencyBackup: %v", err)
	}

	// 模拟 update 后来又新增了一个依赖
	if _, err := pm.InstallToProject(proj, core.Dependency{Name: "mylib", Version: "1.1.0"}); err != nil {
		t.Fatalf("InstallToProject(mylib@1.1.0): %v", err)
	}
	if _, err := pm.InstallToProject(proj, core.Dependency{Name: "other", Version: "1.0.0"}); err != nil {
		t.Fatalf("InstallToProject(other@1.0.0): %v", err)
	}

	backup, err := proj.LoadDependencyBackup()
	if err != nil {
		t.Fatalf("LoadDependencyBackup: %v", err)
	}
	plans := planDependencyRollback(pm, proj, backup)

	var restore, extra *dependencyRollback
	for i := range plans {
		switch plans[i].Name {
		case "mylib":
			restore = &plans[i]
		case "other":
			extra = &plans[i]
		}
	}
	if restore == nil || restore.Action != "restore" || restore.Target != "1.0.0" {
		t.Errorf("mylib rollback plan = %+v, want restore to 1.0.0", restore)
	}
	if extra == nil || extra.Action != "extra" {
		t.Errorf("other rollback plan = %+v, want extra", extra)
	}
}
