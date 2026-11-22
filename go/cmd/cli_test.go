package main

import (
	"os"
	"path/filepath"
	"testing"

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
