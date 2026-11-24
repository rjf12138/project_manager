package core

import (
	"bytes"
	"os"
	"os/exec"
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

	if _, err := os.Stat(filepath.Join(full, ConfigDirName, ConfigFileName)); err != nil {
		t.Fatalf(".pmconfig/config.json missing: %v", err)
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

// 单目标时 generate_type 必须真正决定生成类型（修复 targets 与 generate_type 不一致）。
func TestGenerateCMakeGenerateTypeControlsSingleTarget(t *testing.T) {
	tmp := t.TempDir()
	full, err := CreateProject("GenType", tmp)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}

	cfg := proj.Config()
	cfg.Targets = []Target{{Name: "GenType", Type: "static_lib"}}

	cases := []struct {
		gt   GenerateType
		want string
	}{
		{GenerateTypeExecutable, "add_executable(GenType"},
		{GenerateTypeStaticLib, "add_library(GenType STATIC"},
		{GenerateTypeSharedLib, "add_library(GenType SHARED"},
	}
	for _, tc := range cases {
		cfg.GenerateType = tc.gt
		out := GenerateCMakeLists(cfg, proj.DirInfo())
		if !strings.Contains(out, tc.want) {
			t.Errorf("generate_type=%s should produce %q:\n%s", tc.gt, tc.want, out)
		}
	}
}

// targets 为空时也应按 generate_type 生成对应类型的目标。
func TestGenerateCMakeGenerateTypeWithoutTargets(t *testing.T) {
	tmp := t.TempDir()
	full, err := CreateProject("NoTargetsLib", tmp)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}

	cfg := proj.Config()
	cfg.Targets = nil

	cfg.GenerateType = GenerateTypeStaticLib
	out := GenerateCMakeLists(cfg, proj.DirInfo())
	if !strings.Contains(out, "add_library(NoTargetsLib STATIC") {
		t.Errorf("static lib target missing:\n%s", out)
	}
	if strings.Contains(out, "add_executable(") {
		t.Errorf("unexpected executable target for static_lib:\n%s", out)
	}

	cfg.GenerateType = GenerateTypeSharedLib
	out = GenerateCMakeLists(cfg, proj.DirInfo())
	if !strings.Contains(out, "add_library(NoTargetsLib SHARED") {
		t.Errorf("shared lib target missing:\n%s", out)
	}
}

// 多目标配置属于高级用法，仍由每个 target 的 type 决定，generate_type 不覆盖。
func TestGenerateCMakeMultiTargetsKeepOwnTypes(t *testing.T) {
	tmp := t.TempDir()
	full, err := CreateProject("Multi", tmp)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}

	cfg := proj.Config()
	cfg.GenerateType = GenerateTypeExecutable
	cfg.Targets = []Target{
		{Name: "app", Type: "executable"},
		{Name: "core", Type: "static_lib"},
	}

	out := GenerateCMakeLists(cfg, proj.DirInfo())
	if !strings.Contains(out, "add_executable(app") {
		t.Errorf("executable target missing:\n%s", out)
	}
	if !strings.Contains(out, "add_library(core STATIC") {
		t.Errorf("static lib target missing:\n%s", out)
	}
}

// SaveConfig 保存时同步单目标的类型，避免配置文件里 generate_type 与 targets 长期不一致。
func TestSaveConfigSyncsSingleTargetType(t *testing.T) {
	tmp := t.TempDir()
	full, err := CreateProject("Sync", tmp)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}

	cfg := proj.Config()
	cfg.Targets = []Target{{Name: "Sync", Type: "static_lib"}}
	cfg.GenerateType = GenerateTypeExecutable
	if err := proj.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	reloaded, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}
	got := reloaded.Config()
	if len(got.Targets) != 1 || got.Targets[0].Type != "executable" {
		t.Errorf("targets = %+v, want single executable target", got.Targets)
	}

	out := GenerateCMakeLists(got, reloaded.DirInfo())
	if !strings.Contains(out, "add_executable(Sync") {
		t.Errorf("expected executable after save:\n%s", out)
	}
}

// 新项目配置放在根目录 .pmconfig/ 目录下（config.json），且不创建 config/ 目录。
func TestCreateProjectUsesRootConfigWithoutConfigDir(t *testing.T) {
	tmp := t.TempDir()
	full, err := CreateProject("DotConfig", tmp)
	if err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(full, ConfigDirName, ConfigFileName)
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("%s/%s missing: %v", ConfigDirName, ConfigFileName, err)
	}
	if fi, err := os.Stat(filepath.Join(full, ConfigDirName)); err != nil || !fi.IsDir() {
		t.Errorf("%s should be a directory", ConfigDirName)
	}
	if _, err := os.Stat(filepath.Join(full, "config")); !os.IsNotExist(err) {
		t.Errorf("config/ directory should not be created by pm, stat err = %v", err)
	}
}

// 旧项目（config/.pmproject）仍可加载；保存后迁移到 .pmconfig/config.json。
func TestLoadLegacyConfigAndMigrateOnSave(t *testing.T) {
	tmp := t.TempDir()
	full, err := CreateProject("Legacy", tmp)
	if err != nil {
		t.Fatal(err)
	}

	newCfg := filepath.Join(full, ConfigDirName, ConfigFileName)
	data, err := os.ReadFile(newCfg)
	if err != nil {
		t.Fatal(err)
	}

	// 模拟旧版项目：配置放到 config/.pmproject，移除 .pmconfig 目录
	if err := os.RemoveAll(filepath.Join(full, ConfigDirName)); err != nil {
		t.Fatal(err)
	}
	legacyDir := filepath.Join(full, "config")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatal(err)
	}
	legacyCfg := filepath.Join(legacyDir, LegacyConfigFileName)
	if err := os.WriteFile(legacyCfg, data, 0644); err != nil {
		t.Fatal(err)
	}

	proj, err := LoadProject(full)
	if err != nil {
		t.Fatalf("legacy config should still load: %v", err)
	}
	if proj.Config().Name != "Legacy" {
		t.Errorf("name = %q, want Legacy", proj.Config().Name)
	}

	// 保存配置后应写到新位置，旧文件保留
	if err := proj.SaveConfig(proj.Config()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(newCfg); err != nil {
		t.Errorf("config was not migrated to %s: %v", newCfg, err)
	}
	if _, err := os.Stat(legacyCfg); err != nil {
		t.Errorf("legacy config should be kept for safety: %v", err)
	}
}

// 早期单文件 .pmconfig 格式仍可加载；保存后自动迁移为 .pmconfig/ 目录。
func TestLoadSingleFileConfigAndMigrateOnSave(t *testing.T) {
	tmp := t.TempDir()
	full, err := CreateProject("SingleFile", tmp)
	if err != nil {
		t.Fatal(err)
	}

	cfgDir := filepath.Join(full, ConfigDirName)
	cfgPath := filepath.Join(cfgDir, ConfigFileName)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	// 模拟早期单文件格式：删除目录，把内容写成同名文件
	if err := os.RemoveAll(cfgDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgDir, data, 0644); err != nil {
		t.Fatal(err)
	}

	proj, err := LoadProject(full)
	if err != nil {
		t.Fatalf("single-file config should still load: %v", err)
	}

	if err := proj.SaveConfig(proj.Config()); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(cfgDir); err != nil || !fi.IsDir() {
		t.Errorf("%s should be migrated to a directory", ConfigDirName)
	}
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("%s/%s missing after migration: %v", ConfigDirName, ConfigFileName, err)
	}
}

// ===================== 构建前后脚本（.pmconfig/*.sh） =====================

// 构建脚本按文件内容执行，输出带阶段前缀。
func TestRunBuildScript(t *testing.T) {
	root := t.TempDir()
	proj := NewProject()
	proj.fillDirInfo(root)

	cfgDir := projectConfigDir(root)
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(cfgDir, PreBuildScriptName)
	script := "#!/bin/sh\necho a >> order.txt\necho b >> order.txt\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := proj.runBuildScript("pre-build", PreBuildScriptName, &out, &out); err != nil {
		t.Fatalf("runBuildScript: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "order.txt"))
	if err != nil {
		t.Fatalf("order.txt missing: %v", err)
	}
	if got := string(data); got != "a\nb\n" {
		t.Errorf("script did not run as expected: %q", got)
	}
	if !strings.Contains(out.String(), "[pre-build] "+scriptPath) {
		t.Errorf("missing stage prefix in output:\n%s", out.String())
	}
}

// 脚本文件不存在时静默跳过。
func TestRunBuildScriptMissingIsSkipped(t *testing.T) {
	root := t.TempDir()
	proj := NewProject()
	proj.fillDirInfo(root)

	var out bytes.Buffer
	if err := proj.runBuildScript("pre-build", PreBuildScriptName, &out, &out); err != nil {
		t.Fatalf("missing script should be skipped: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("unexpected output for missing script: %q", out.String())
	}
}

// 构建脚本失败时返回错误并带上脚本路径。
func TestRunBuildScriptFailure(t *testing.T) {
	root := t.TempDir()
	proj := NewProject()
	proj.fillDirInfo(root)

	cfgDir := projectConfigDir(root)
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(cfgDir, PostBuildScriptName)
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 5\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := proj.runBuildScript("post-build", PostBuildScriptName, &out, &out)
	if err == nil {
		t.Fatal("expected error for failing script")
	}
	if !strings.Contains(err.Error(), "post-build script failed") {
		t.Errorf("error = %q, want post-build failure", err)
	}
}

// pre_build.sh 失败必须中止构建（不会走到 cmake）。
func TestPreBuildFailureStopsBuild(t *testing.T) {
	tmp := t.TempDir()
	full, err := CreateProject("PreFail", tmp)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(full, ConfigDirName, PreBuildScriptName)
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 7\n"), 0644); err != nil {
		t.Fatal(err)
	}

	log, err := proj.BuildCapture(false)
	if err == nil {
		t.Fatalf("build should fail when pre_build.sh fails\n%s", log)
	}
	if !strings.Contains(err.Error(), "pre-build script failed") {
		t.Errorf("error = %q, want pre-build failure", err)
	}
	if !strings.Contains(log, "[pre-build] "+scriptPath) {
		t.Errorf("pre-build output missing from log:\n%s", log)
	}
}

// 端到端：pre_build.sh / post_build.sh 都在编译流程中执行，且顺序正确。
func TestBuildHooksRunInOrder(t *testing.T) {
	if _, err := exec.LookPath("cmake"); err != nil {
		t.Skip("cmake not installed")
	}

	tmp := t.TempDir()
	full, err := CreateProject("Hooked", tmp)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := LoadProject(full)
	if err != nil {
		t.Fatal(err)
	}

	cfgDir := filepath.Join(full, ConfigDirName)
	preScript := filepath.Join(cfgDir, PreBuildScriptName)
	postScript := filepath.Join(cfgDir, PostBuildScriptName)
	if err := os.WriteFile(preScript, []byte("#!/bin/sh\necho pre >> hook_order.txt\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(postScript, []byte("#!/bin/sh\necho post >> hook_order.txt\n"), 0644); err != nil {
		t.Fatal(err)
	}

	log, err := proj.BuildCapture(false)
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, log)
	}
	for _, want := range []string{"[pre-build] " + preScript, "[post-build] " + postScript} {
		if !strings.Contains(log, want) {
			t.Errorf("build log missing %q:\n%s", want, log)
		}
	}

	data, err := os.ReadFile(filepath.Join(full, "hook_order.txt"))
	if err != nil {
		t.Fatalf("hook_order.txt missing: %v", err)
	}
	if got := string(data); got != "pre\npost\n" {
		t.Errorf("hooks ran out of order: %q", got)
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
