package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rjf12138/project_manager/core"
)

// ===================== 测试辅助 =====================

func keyMsg(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func runeMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// isQuit 判断命令是否为 tea.Quit（quitMsg 未导出，只能按类型名判断）
func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	if msg == nil {
		return false
	}
	return reflect.TypeOf(msg).Name() == "quitMsg"
}

// newTestModel 构造一个与真实环境隔离的 Solution（HOME 指向临时目录），
// 并创建一个可用的示例项目。
func newTestModel(t *testing.T) (MainModel, string) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())
	s := core.NewSolution()
	if err := s.Init(); err != nil {
		t.Fatalf("Solution.Init: %v", err)
	}

	path, err := core.CreateProject("demo", t.TempDir())
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := s.LoadProject(path); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}

	m := NewMainModel(s)
	// 隔离随二进制分发的内置包，保证测试只依赖自己造的仓库内容
	m.pm.SetBuiltinDir("")

	return m, path
}

// newTestModelWithPkg 在上面基础上再往本地仓库放一个 mylib@1.0.0 包
func newTestModelWithPkg(t *testing.T) (MainModel, *core.Project) {
	t.Helper()

	m, _ := newTestModel(t)
	active := m.solution.ActiveProject()
	if active == nil {
		t.Fatal("expected an active project")
	}

	pm := core.NewPackageManager()
	if err := pm.Init(); err != nil {
		t.Fatalf("pm.Init: %v", err)
	}

	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "manifest.json"),
		`{"name":"mylib","version":"1.0.0","description":"test lib","link_libs":["mylib"]}`)
	mustWrite(t, filepath.Join(src, "include", "mylib.h"), "// mylib\n")
	mustWrite(t, filepath.Join(src, "lib", "debug", "libmylib.a"), "debug")
	mustWrite(t, filepath.Join(src, "lib", "release", "libmylib.a"), "release")
	if _, err := pm.CreatePackage(src, ""); err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}

	return m, active
}

// ===================== 主菜单 =====================

// 主菜单的帮助文字写着 q quit，此前 q 在主菜单上没有任何反应。
func TestMainMenuQuitsWithQ(t *testing.T) {
	m, _ := newTestModel(t)

	_, cmd := m.Update(runeMsg("q"))
	if !isQuit(cmd) {
		t.Fatalf("q on main menu should quit the program")
	}
}

// 主菜单选择 "Load / Open Project" 应该进入加载页面，而不是空操作。
func TestMainMenuLoadOpensLoadPage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := core.NewSolution()
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}

	m := NewMainModel(s)
	model, _ := m.Update(keyMsg(tea.KeyEnter))
	got := model.(MainModel)
	if got.page != PageLoadProject {
		t.Fatalf("page = %v, want PageLoadProject", got.page)
	}
}

// 在加载页输入路径并回车应能注册项目。
func TestLoadProjectFromTUI(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := core.NewSolution()
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}

	projPath, err := core.CreateProject("loaded", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	m := NewMainModel(s)
	model, _ := m.Update(keyMsg(tea.KeyEnter)) // 进入加载页
	model, _ = model.Update(runeMsg(projPath)) // 输入路径
	model, _ = model.Update(keyMsg(tea.KeyEnter))

	got := model.(MainModel)
	if got.solution.Size() != 1 {
		t.Fatalf("solution size = %d, want 1 (status=%q)", got.solution.Size(), got.statusMsg)
	}
	if got.page != PageMainMenu {
		t.Errorf("page = %v, want PageMainMenu", got.page)
	}
	if got.solution.ActiveProjectPath() != projPath {
		t.Errorf("active = %q, want %q", got.solution.ActiveProjectPath(), projPath)
	}
}

// 加载不存在的路径应给出错误提示并停留在当前页。
func TestLoadProjectRejectsBadPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := core.NewSolution()
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}

	m := NewMainModel(s)
	model, _ := m.Update(keyMsg(tea.KeyEnter))
	model, _ = model.Update(runeMsg("/definitely/not/here"))
	model, _ = model.Update(keyMsg(tea.KeyEnter))

	got := model.(MainModel)
	if got.page != PageLoadProject {
		t.Errorf("page = %v, want PageLoadProject", got.page)
	}
	if got.statusType != "error" {
		t.Errorf("statusType = %q, want error", got.statusType)
	}
	if got.solution.Size() != 0 {
		t.Errorf("solution size = %d, want 0", got.solution.Size())
	}
}

// ===================== 构建流程 =====================

// 构建时必须同时启动 spinner，否则界面上的动画是静止的。
func TestBuildStartsSpinnerTick(t *testing.T) {
	m, _ := newTestModel(t)
	m.cursor = 3 // Rebuild

	model, cmd := m.Update(keyMsg(tea.KeyEnter))
	got := model.(MainModel)

	if !got.loading {
		t.Fatalf("loading = false, want true after starting a build")
	}
	if cmd == nil {
		t.Fatalf("expected a command")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", cmd())
	}
	if len(batch) < 2 {
		t.Fatalf("expected build cmd + spinner tick, got %d command(s)", len(batch))
	}
}

// 构建结束后进入日志页，并默认滚动到末尾。
func TestBuildDoneShowsLogTail(t *testing.T) {
	m, _ := newTestModel(t)

	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "line")
	}
	log := ""
	for _, l := range lines {
		log += l + "\n"
	}

	model, _ := m.Update(buildDoneMsg{log: log})
	got := model.(MainModel)

	if got.page != PageBuildLog {
		t.Fatalf("page = %v, want PageBuildLog", got.page)
	}
	if got.loading {
		t.Errorf("loading = true, want false")
	}
	if got.logOffset != got.maxLogOffset() {
		t.Errorf("logOffset = %d, want tail %d", got.logOffset, got.maxLogOffset())
	}
}

// 日志页 esc 返回构建页，且不会误触发退出。
func TestBuildLogEscReturnsToBuildPage(t *testing.T) {
	m, _ := newTestModel(t)
	m.page = PageBuildLog
	m.buildLog = []string{"a", "b"}

	model, cmd := m.Update(keyMsg(tea.KeyEsc))
	got := model.(MainModel)

	if got.page != PageBuild {
		t.Errorf("page = %v, want PageBuild", got.page)
	}
	if isQuit(cmd) {
		t.Errorf("esc on build log should not quit")
	}
}

// ===================== 配置编辑 =====================

// setEditList 使用值接收者时修改会丢失，这里锁定回写行为。
func TestApplyEditValuePersists(t *testing.T) {
	m, _ := newTestModel(t)
	m.page = PageSrcDirsEdit
	m.cfgEditMode = "src_dirs"
	m.editingCfg.SourceDirs = []string{"src"}
	m.cfgEditCursor = 1 // 指向 "Add New"

	m.applyEditValue("tests")

	got := m.editingCfg.SourceDirs
	if len(got) != 2 || got[1] != "tests" {
		t.Fatalf("SourceDirs = %v, want [src tests]", got)
	}
	if !m.cfgModified {
		t.Errorf("cfgModified = false, want true")
	}
}

// 内联编辑的删除键必须按字符处理，不能把多字节字符切成半个字节。
func TestInlineEditBackspaceKeepsUTF8(t *testing.T) {
	m, _ := newTestModel(t)
	m.page = PageIncDirsEdit
	m.cfgEditMode = "inc_dirs"
	m.cfgEditFocus = 1

	model, _ := m.Update(runeMsg("中文路径"))
	model, _ = model.Update(keyMsg(tea.KeyBackspace))

	got := model.(MainModel)
	if s := string(got.cfgEditRunes); s != "中文路" {
		t.Fatalf("after backspace = %q, want %q", s, "中文路")
	}
	for _, r := range got.cfgEditRunes {
		if r == '\uFFFD' {
			t.Fatalf("input contains invalid utf-8: %q", string(got.cfgEditRunes))
		}
	}
}

// ctrl+c 在编辑输入模式下也必须能退出。
func TestCtrlCQuitsWhileTyping(t *testing.T) {
	m, _ := newTestModel(t)
	m.page = PageIncDirsEdit
	m.cfgEditFocus = 1
	m.cfgEditRunes = []rune("typing")

	_, cmd := m.Update(keyMsg(tea.KeyCtrlC))
	if !isQuit(cmd) {
		t.Fatalf("ctrl+c should quit even while typing")
	}
}

// 列表项删除后游标不能越界。
func TestCfgEditDeleteKeepsCursorInRange(t *testing.T) {
	m, _ := newTestModel(t)
	m.page = PageSrcDirsEdit
	m.cfgEditMode = "src_dirs"
	m.editingCfg.SourceDirs = []string{"src", "tests"}
	m.cfgEditCursor = 1

	model, _ := m.Update(runeMsg("d"))
	got := model.(MainModel)

	if len(got.editingCfg.SourceDirs) != 1 || got.editingCfg.SourceDirs[0] != "src" {
		t.Fatalf("SourceDirs = %v, want [src]", got.editingCfg.SourceDirs)
	}
	if got.cfgEditCursor != 0 {
		t.Errorf("cursor = %d, want 0", got.cfgEditCursor)
	}
}

// ===================== 直接模式 (-cfg / -l) =====================

func TestDirectModeEscQuitsFromConfig(t *testing.T) {
	m, _ := newTestModel(t)
	m.directMode = true
	m.page = PageConfig

	_, cmd := m.Update(keyMsg(tea.KeyEsc))
	if !isQuit(cmd) {
		t.Fatalf("esc in -cfg direct mode should quit")
	}
}

func TestDirectModeSubPageQQuits(t *testing.T) {
	m, _ := newTestModel(t)
	m.directMode = true
	m.page = PageSrcDirsEdit
	m.cfgEditMode = "src_dirs"

	_, cmd := m.Update(runeMsg("q"))
	if !isQuit(cmd) {
		t.Fatalf("q on config sub-page in -cfg mode should quit")
	}
}

// 普通模式下子页面 esc 返回主菜单。
func TestNormalModeEscReturnsToMainMenu(t *testing.T) {
	m, _ := newTestModel(t)
	m.page = PageBuild

	model, cmd := m.Update(keyMsg(tea.KeyEsc))
	got := model.(MainModel)

	if got.page != PageMainMenu {
		t.Errorf("page = %v, want PageMainMenu", got.page)
	}
	if isQuit(cmd) {
		t.Errorf("esc outside direct mode should not quit")
	}
}

// pm -l 直接模式下，回车选择项目：切换活动项目、退出，并记录待打开路径。
func TestDirectModeListEnterRecordsOpenPathAndQuits(t *testing.T) {
	m, projPath := newTestModel(t)
	m.directMode = true
	m.page = PageList
	m.cursor = 0

	model, cmd := m.Update(keyMsg(tea.KeyEnter))
	got := model.(MainModel)

	if !isQuit(cmd) {
		t.Fatalf("enter in -l direct mode should quit the program")
	}
	if got.solution.ActiveProjectPath() != projPath {
		t.Errorf("active project = %q, want %q", got.solution.ActiveProjectPath(), projPath)
	}
	if got.openInEditorPath != projPath {
		t.Errorf("openInEditorPath = %q, want %q", got.openInEditorPath, projPath)
	}
}

// 完整 TUI 的项目列表回车只切换活动项目并返回主菜单，不记录打开路径。
func TestListEnterSwitchesActiveWithoutOpen(t *testing.T) {
	m, projPath := newTestModel(t)
	m.page = PageList
	m.cursor = 0

	model, cmd := m.Update(keyMsg(tea.KeyEnter))
	got := model.(MainModel)

	if got.page != PageMainMenu {
		t.Errorf("page = %v, want PageMainMenu", got.page)
	}
	if isQuit(cmd) {
		t.Errorf("enter outside direct mode should not quit")
	}
	if got.openInEditorPath != "" {
		t.Errorf("openInEditorPath = %q, want empty", got.openInEditorPath)
	}
	if got.solution.ActiveProjectPath() != projPath {
		t.Errorf("active project = %q, want %q", got.solution.ActiveProjectPath(), projPath)
	}
}

// ===================== 编辑器打开（pm -l 退出后） =====================

// 找不到编辑器命令时应返回错误，而不是静默失败。
func TestOpenInEditorCommandNotFound(t *testing.T) {
	t.Setenv("PM_EDITOR", "")
	t.Setenv("PATH", t.TempDir()) // 空目录，确保没有 code

	err := openInEditor("/some/project")
	if err == nil {
		t.Fatal("expected error when editor command is missing")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("error = %q, want 'not found in PATH'", err)
	}
}

// 默认编辑器（code）存在时会被调用，并收到项目路径参数。
func TestOpenInEditorLaunchesCode(t *testing.T) {
	binDir, marker := t.TempDir(), filepath.Join(t.TempDir(), "args.txt")
	writeFakeEditor(t, binDir, "code", marker)

	t.Setenv("PM_EDITOR", "")
	t.Setenv("PATH", binDir)

	proj := "/tmp/fake-project"
	if err := openInEditor(proj); err != nil {
		t.Fatalf("openInEditor: %v", err)
	}

	got := waitForEditorCall(t, marker)
	if strings.TrimSpace(got) != proj {
		t.Errorf("editor args = %q, want %q", got, proj)
	}
}

// PM_EDITOR 可覆盖默认命令，附加参数按顺序传在项目路径之前。
func TestOpenInEditorRespectsPMEditor(t *testing.T) {
	binDir, marker := t.TempDir(), filepath.Join(t.TempDir(), "args.txt")
	writeFakeEditor(t, binDir, "myeditor", marker)

	t.Setenv("PM_EDITOR", "myeditor --new-window")
	t.Setenv("PATH", binDir)

	proj := "/tmp/another-project"
	if err := openInEditor(proj); err != nil {
		t.Fatalf("openInEditor: %v", err)
	}

	got := strings.TrimSpace(waitForEditorCall(t, marker))
	if want := "--new-window " + proj; got != want {
		t.Errorf("editor args = %q, want %q", got, want)
	}
}

// writeFakeEditor 在 binDir 写入一个名为 name 的可执行脚本，调用时把全部参数写到 marker。
func writeFakeEditor(t *testing.T, binDir, name, marker string) {
	t.Helper()
	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" > '%s'\n", marker)
	mustWrite(t, filepath.Join(binDir, name), script)
	if err := os.Chmod(filepath.Join(binDir, name), 0755); err != nil {
		t.Fatal(err)
	}
}

// waitForEditorCall 轮询等待编辑器脚本写入参数，超时则失败。
func waitForEditorCall(t *testing.T, marker string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(marker); err == nil {
			return string(b)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("editor was not invoked (no %s written within timeout)", marker)
	return ""
}

// 未构建时运行应给出错误提示，而不是后台起一个 goroutine 后丢失输出。
func TestRunWithoutExecutableReportsError(t *testing.T) {
	m, _ := newTestModel(t)
	active := m.solution.ActiveProject()
	if active == nil {
		t.Fatal("expected an active project")
	}

	model, cmd := m.startRun(active)
	got := model.(MainModel)

	if cmd != nil {
		t.Errorf("expected no command when executable is missing")
	}
	if got.statusType != "error" || !strings.Contains(got.statusMsg, "executable not found") {
		t.Errorf("status = %q/%q, want executable-not-found error", got.statusType, got.statusMsg)
	}
}

// ===================== 渲染 =====================

// 每个页面都必须能正常渲染（防止越界 / 空指针）。
func TestRenderAllPages(t *testing.T) {
	m, _ := newTestModel(t)
	m.ready = true
	m.width, m.height = 100, 40

	pages := []Page{
		PageMainMenu,
		PageCreateProject,
		PageLoadProject,
		PageBuild,
		PageBuildLog,
		PageConfig,
		PageList,
		PageDepsEdit,
		PageLibDirsEdit,
		PageIncDirsEdit,
		PageSrcDirsEdit,
		PageDepsList,
		PagePkgList,
		PagePkgVersions,
	}

	for _, p := range pages {
		m.page = p
		if out := m.View(); out == "" {
			t.Errorf("page %v rendered empty view", p)
		}
	}

	// 空解决方案的主菜单 / 列表页
	empty, _ := newEmptyModel(t)
	empty.ready = true
	empty.page = PageMainMenu
	if out := empty.View(); out == "" {
		t.Errorf("empty main menu rendered empty view")
	}
	empty.page = PageList
	if out := empty.View(); out == "" {
		t.Errorf("empty list page rendered empty view")
	}
}

// 日志页在游标越界（例如窗口变小）时也要能安全渲染。
func TestBuildLogRenderClampsOffset(t *testing.T) {
	m, _ := newTestModel(t)
	m.ready = true
	m.height = 20
	m.page = PageBuildLog
	m.buildLog = []string{"a", "b", "c"}
	m.logOffset = 9999

	if out := m.View(); out == "" {
		t.Fatalf("build log page rendered empty view")
	}
}

func newEmptyModel(t *testing.T) (MainModel, *core.Solution) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	s := core.NewSolution()
	if err := s.Init(); err != nil {
		t.Fatalf("Solution.Init: %v", err)
	}
	m := NewMainModel(s)
	m.pm.SetBuiltinDir("")
	return m, s
}

// ===================== 依赖管理页 =====================

// 配置页的 Dependencies 行能进入依赖列表
func TestConfigDependenciesRowOpensList(t *testing.T) {
	m, _ := newTestModel(t)
	m.page = PageConfig
	m.cursor = configRowDependencies

	model, _ := m.Update(keyMsg(tea.KeyEnter))
	got := model.(MainModel)

	if got.page != PageDepsList {
		t.Fatalf("page = %v, want PageDepsList", got.page)
	}
}

// 依赖列表能显示包名、版本与安装状态
func TestDepsListShowsStatus(t *testing.T) {
	m, active := newTestModelWithPkg(t)
	if _, err := m.pm.InstallToProject(active, core.Dependency{Name: "mylib", Version: "1.0.0"}); err != nil {
		t.Fatalf("InstallToProject: %v", err)
	}

	m.ready = true
	m.page = PageDepsList
	out := m.View()

	for _, want := range []string{"Dependencies", "mylib", "1.0.0", "installed"} {
		if !strings.Contains(out, want) {
			t.Errorf("deps page missing %q:\n%s", want, out)
		}
	}

	// 删掉库文件后状态应变为 headers-only
	if err := os.Remove(filepath.Join(active.DirInfo().LibDebug, "libmylib.a")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(active.DirInfo().LibRelease, "libmylib.a")); err != nil {
		t.Fatal(err)
	}
	if out := m.View(); !strings.Contains(out, "headers-only") {
		t.Errorf("expected headers-only status, got:\n%s", out)
	}
}

// 在 TUI 里从本地仓库安装包：写完配置并落盘文件
func TestInstallPackageFromTUI(t *testing.T) {
	m, active := newTestModelWithPkg(t)

	// 配置页 → Dependencies
	m.page = PageConfig
	m.cursor = configRowDependencies
	model, _ := m.Update(keyMsg(tea.KeyEnter))
	got := model.(MainModel)

	// a → 安装页
	model, _ = got.Update(runeMsg("a"))
	got = model.(MainModel)
	if got.page != PagePkgList {
		t.Fatalf("page = %v, want PagePkgList", got.page)
	}
	if len(got.repoPkgs) != 1 {
		t.Fatalf("repoPkgs = %d, want 1", len(got.repoPkgs))
	}

	// enter → 安装
	model, _ = got.Update(keyMsg(tea.KeyEnter))
	got = model.(MainModel)

	if got.page != PageDepsList {
		t.Errorf("page = %v, want PageDepsList after install", got.page)
	}
	if got.statusType != "success" {
		t.Errorf("status = %q/%q, want success", got.statusType, got.statusMsg)
	}

	cfg := active.Config()
	if len(cfg.Dependencies) != 1 || cfg.Dependencies[0].Name != "mylib" {
		t.Fatalf("dependencies = %+v, want mylib", cfg.Dependencies)
	}
	if libs := cfg.LegacyDeps.Libs; len(libs) != 1 || libs[0] != "mylib" {
		t.Errorf("deps.libs = %v, want [mylib]", libs)
	}
	if _, err := os.Stat(filepath.Join(active.DirInfo().ExternInc, "mylib", "mylib.h")); err != nil {
		t.Errorf("headers not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(active.DirInfo().LibDebug, "libmylib.a")); err != nil {
		t.Errorf("lib not installed: %v", err)
	}
}

// 在 TUI 里卸载依赖：删声明并清理文件
// 安装页默认按包名聚合；v 进入版本页并安装选中版本
func TestPkgVersionsPageInstallsSelectedVersion(t *testing.T) {
	m, active := newTestModelWithPkg(t)

	// 再放一个 mylib@1.1.0
	pm := core.NewPackageManager()
	if err := pm.Init(); err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	mustWrite(t, filepath.Join(src, "manifest.json"),
		`{"name":"mylib","version":"1.1.0","description":"test lib v2","link_libs":["mylib"]}`)
	mustWrite(t, filepath.Join(src, "include", "mylib.h"), "// mylib v2\n")
	mustWrite(t, filepath.Join(src, "lib", "debug", "libmylib.a"), "debug-v2")
	mustWrite(t, filepath.Join(src, "lib", "release", "libmylib.a"), "release-v2")
	if _, err := pm.CreatePackage(src, ""); err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}

	model, _ := m.enterPkgList()
	got := model.(MainModel)
	if len(got.repoSummaries) != 1 {
		t.Fatalf("repoSummaries = %d, want 1", len(got.repoSummaries))
	}
	if latest := got.repoSummaries[0].Latest.Manifest.Version; latest != "1.1.0" {
		t.Fatalf("latest = %s, want 1.1.0", latest)
	}

	got.ready = true
	out := got.View()
	for _, want := range []string{"mylib", "1.1.0", "2 versions"} {
		if !strings.Contains(out, want) {
			t.Errorf("collapsed package list missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "1.0.0") {
		t.Errorf("collapsed package list should not show the old version:\n%s", out)
	}

	// v → 版本页，两个版本都可见
	model, _ = got.Update(runeMsg("v"))
	got = model.(MainModel)
	if got.page != PagePkgVersions {
		t.Fatalf("page = %v, want PagePkgVersions", got.page)
	}
	if len(got.repoPkgs) != 2 {
		t.Fatalf("repoPkgs = %d, want 2", len(got.repoPkgs))
	}
	out = got.View()
	for _, want := range []string{"1.1.0", "1.0.0"} {
		if !strings.Contains(out, want) {
			t.Errorf("version page missing %q:\n%s", want, out)
		}
	}

	// 选中第二个版本 1.0.0 并安装
	model, _ = got.Update(keyMsg(tea.KeyDown))
	got = model.(MainModel)
	model, _ = got.Update(keyMsg(tea.KeyEnter))
	got = model.(MainModel)

	if got.page != PageDepsList {
		t.Fatalf("page = %v, want PageDepsList after install", got.page)
	}
	if got.statusType != "success" {
		t.Fatalf("status = %q/%q, want success", got.statusType, got.statusMsg)
	}
	var installed string
	for _, d := range active.Config().Dependencies {
		if d.Name == "mylib" {
			installed = d.Version
		}
	}
	if installed != "1.0.0" {
		t.Errorf("installed mylib version = %q, want 1.0.0", installed)
	}
}

func TestUninstallFromTUI(t *testing.T) {
	m, active := newTestModelWithPkg(t)
	if _, err := m.pm.InstallToProject(active, core.Dependency{Name: "mylib", Version: "1.0.0"}); err != nil {
		t.Fatalf("InstallToProject: %v", err)
	}

	m.page = PageDepsList
	m.depsCursor = 0

	model, _ := m.Update(runeMsg("d"))
	got := model.(MainModel)

	if got.statusType != "success" {
		t.Fatalf("status = %q/%q, want success", got.statusType, got.statusMsg)
	}
	if len(active.Config().Dependencies) != 0 {
		t.Errorf("dependencies = %+v, want empty", active.Config().Dependencies)
	}
	if len(active.Config().LegacyDeps.Libs) != 0 {
		t.Errorf("deps.libs = %v, want empty", active.Config().LegacyDeps.Libs)
	}
	for _, p := range []string{
		filepath.Join(active.DirInfo().ExternInc, "mylib"),
		filepath.Join(active.DirInfo().LibDebug, "libmylib.a"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should be removed, stat err = %v", p, err)
		}
	}
}

// u 重新安装全部依赖，能修复被删掉的文件
func TestReinstallAllFromTUI(t *testing.T) {
	m, active := newTestModelWithPkg(t)
	if _, err := m.pm.InstallToProject(active, core.Dependency{Name: "mylib", Version: "1.0.0"}); err != nil {
		t.Fatalf("InstallToProject: %v", err)
	}

	incFile := filepath.Join(active.DirInfo().ExternInc, "mylib", "mylib.h")
	if err := os.RemoveAll(filepath.Join(active.DirInfo().ExternInc, "mylib")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(incFile); !os.IsNotExist(err) {
		t.Fatal("setup: header should be gone")
	}

	m.page = PageDepsList
	model, _ := m.Update(runeMsg("u"))
	got := model.(MainModel)

	if got.statusType != "success" {
		t.Fatalf("status = %q/%q, want success", got.statusType, got.statusMsg)
	}
	if _, err := os.Stat(incFile); err != nil {
		t.Errorf("reinstall did not restore headers: %v", err)
	}
}

// 卸载后游标不能越界
func TestDepsListCursorAfterUninstall(t *testing.T) {
	m, active := newTestModelWithPkg(t)
	if _, err := m.pm.InstallToProject(active, core.Dependency{Name: "mylib", Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}

	m.page = PageDepsList
	m.depsCursor = 0

	model, _ := m.Update(runeMsg("d"))
	got := model.(MainModel)
	if got.depsCursor != 0 {
		t.Errorf("depsCursor = %d, want 0", got.depsCursor)
	}
	// 空列表下继续操作不应 panic
	model, _ = got.Update(runeMsg("d"))
	got = model.(MainModel)
	if got.page != PageDepsList {
		t.Errorf("page = %v, want PageDepsList", got.page)
	}
}

// 依赖页负责刷新：安装后 editingCfg 应与磁盘一致
func TestInstallRefreshesEditingCfg(t *testing.T) {
	m, active := newTestModelWithPkg(t)
	_ = active

	m.page = PageConfig
	m.cursor = configRowDependencies
	model, _ := m.Update(keyMsg(tea.KeyEnter))
	model, _ = model.(MainModel).Update(runeMsg("a"))
	model, _ = model.(MainModel).Update(keyMsg(tea.KeyEnter))

	got := model.(MainModel)
	if len(got.editingCfg.Dependencies) != 1 {
		t.Errorf("editingCfg.Dependencies = %+v, want 1 entry", got.editingCfg.Dependencies)
	}
	if got.cfgModified {
		t.Errorf("cfgModified = true, want false after install")
	}
}
