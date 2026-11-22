package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateProjectSkeleton(t *testing.T) {
	tmp := t.TempDir()

	full, err := CreateProject("Demo", tmp)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if _, err := os.Stat(filepath.Join(full, "config", ".pmproject")); err != nil {
		t.Fatalf("config/.pmproject missing: %v", err)
	}

	dirs := []string{
		"doc", "extern_inc", "inc", "src", "main",
		"lib/debug", "lib/release",
		"output/debug/bin", "output/debug/lib",
		"output/release/bin", "output/release/lib",
	}
	for _, d := range dirs {
		fi, err := os.Stat(filepath.Join(full, d))
		if err != nil || !fi.IsDir() {
			t.Errorf("directory %s missing", d)
		}
	}

	proj, err := LoadProject(full)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	cfg := proj.Config()
	if cfg.Name != "Demo" || cfg.UUID == "" {
		t.Errorf("unexpected config: name=%q uuid=%q", cfg.Name, cfg.UUID)
	}
	if cfg.Path != full {
		t.Errorf("config path = %q, want %q", cfg.Path, full)
	}
}

func TestGenerateCMakeUsesBuildOptions(t *testing.T) {
	tmp := t.TempDir()
	full, err := CreateProject("Cm", tmp)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}

	cfg := proj.Config()
	cfg.BuildMode = BuildModeRelease
	cfg.BuildOptions.Release = BuildModeOptions{
		Defines:      []string{"MYFLAG"},
		Optimization: "-O2",
	}

	out := GenerateCMakeLists(cfg, proj.DirInfo())
	if !strings.Contains(out, "-DMYFLAG") {
		t.Errorf("release defines not applied:\n%s", out)
	}
	if !strings.Contains(out, "-O2") {
		t.Errorf("release optimization not applied")
	}
	if strings.Contains(out, "-O0") {
		t.Errorf("debug optimization leaked into release output")
	}
	if !strings.Contains(out, "output/release/bin") {
		t.Errorf("release output dir not applied")
	}

	cfg.BuildMode = BuildModeDebug
	cfg.BuildOptions.Debug = BuildModeOptions{Defines: []string{"DBGFLAG"}, Optimization: "-Og"}
	outDebug := GenerateCMakeLists(cfg, proj.DirInfo())
	if !strings.Contains(outDebug, "-DDBGFLAG") || !strings.Contains(outDebug, "-Og") {
		t.Errorf("debug defines/optimization not applied:\n%s", outDebug)
	}
	if !strings.Contains(outDebug, "output/debug/bin") {
		t.Errorf("debug output dir not applied")
	}
}

func TestAddAndRemoveDependencyPersists(t *testing.T) {
	tmp := t.TempDir()
	full, err := CreateProject("Dep", tmp)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}

	if err := proj.AddDependency(Dependency{Name: "foo", Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}
	if err := proj.AddLinkLibs("foo"); err != nil {
		t.Fatal(err)
	}

	reloaded, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Config().Dependencies) != 1 {
		t.Fatalf("dependency not persisted: %+v", reloaded.Config().Dependencies)
	}
	if len(reloaded.Config().LegacyDeps.Libs) != 1 {
		t.Fatalf("link lib not persisted: %+v", reloaded.Config().LegacyDeps.Libs)
	}

	if err := reloaded.RemoveDependencyByName("foo"); err != nil {
		t.Fatal(err)
	}

	final, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}
	if len(final.Config().Dependencies) != 0 {
		t.Errorf("dependency not removed: %+v", final.Config().Dependencies)
	}
	if len(final.Config().LegacyDeps.Libs) != 0 {
		t.Errorf("link lib not removed: %+v", final.Config().LegacyDeps.Libs)
	}
}

func TestGenerateCMakeLinksDepsWithoutTargets(t *testing.T) {
	tmp := t.TempDir()
	full, err := CreateProject("NoTarget", tmp)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}

	cfg := proj.Config()
	cfg.Targets = nil
	cfg.LegacyDeps.Libs = []string{"greeter"}

	out := GenerateCMakeLists(cfg, proj.DirInfo())
	if !strings.Contains(out, "add_executable(NoTarget") {
		t.Errorf("default executable target missing:\n%s", out)
	}
	if !strings.Contains(out, "target_link_libraries(NoTarget") || !strings.Contains(out, "greeter") {
		t.Errorf("dependency libs not linked for default target:\n%s", out)
	}
}

func TestScanMainFiles(t *testing.T) {
	tmp := t.TempDir()
	full, err := CreateProject("Scan", tmp)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}

	mainDir := proj.DirInfo().Main
	for _, name := range []string{"a.cc", "b.cpp", "c.c", "ignore.h", "note.txt"} {
		if err := os.WriteFile(filepath.Join(mainDir, name), []byte("//\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := proj.ScanMainFiles()
	if err != nil {
		t.Fatal(err)
	}

	// 新建项目自带 main.cc，加上上面写入的 3 个源文件
	want := map[string]bool{"main.cc": true, "a.cc": true, "b.cpp": true, "c.c": true}
	if len(files) != len(want) {
		t.Errorf("expected %d source files, got %v", len(want), files)
	}
	got := map[string]bool{}
	for _, f := range files {
		got[f] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("source file %s not scanned: %v", name, files)
		}
	}
	for _, name := range []string{"ignore.h", "note.txt"} {
		if got[name] {
			t.Errorf("non-source file %s should be ignored: %v", name, files)
		}
	}
}
