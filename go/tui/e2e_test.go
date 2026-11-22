package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rjf12138/project_manager/core"
)

// TestProgramCreateProjectEndToEnd 通过真实的 tea.Program 事件循环驱动界面，
// 覆盖“主菜单 -> 创建页 -> 填表 -> 回到主菜单”的完整操作链路。
// 使用 p.Send 注入按键，避免依赖终端输入解析，结果稳定可重复。
func TestProgramCreateProjectEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping program end-to-end test in short mode")
	}

	t.Setenv("HOME", t.TempDir())
	s := core.NewSolution()
	if err := s.Init(); err != nil {
		t.Fatalf("Solution.Init: %v", err)
	}
	parent := t.TempDir()

	m := NewMainModel(s)
	m.ready = true
	m.width, m.height = 100, 32

	var out bytes.Buffer
	p := tea.NewProgram(m, tea.WithInput(strings.NewReader("")), tea.WithOutput(&out))

	key := func(t tea.KeyType) tea.Msg { return tea.KeyMsg{Type: t} }
	runes := func(v string) tea.Msg {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(v)}
	}

	script := []tea.Msg{
		key(tea.KeyEnter), // Load / Open Project
		key(tea.KeyEsc),   // 返回主菜单
		key(tea.KeyDown),  // 移动光标到 Create New Project
		key(tea.KeyEnter), // 进入创建页
		runes("demo"),     // 项目名
		key(tea.KeyDown),  // 切换到父目录输入框
		key(tea.KeyBackspace),
		runes(parent),
		key(tea.KeyEnter), // 创建
	}

	go func() {
		for _, msg := range script {
			p.Send(msg)
		}
		p.Quit()
	}()

	final, err := p.Run()
	if err != nil {
		t.Fatalf("program run: %v", err)
	}

	got, ok := final.(MainModel)
	if !ok {
		t.Fatalf("final model type = %T, want MainModel", final)
	}
	if got.page != PageMainMenu {
		t.Errorf("final page = %v, want PageMainMenu", got.page)
	}
	if got.statusMsg != "Created: demo" {
		t.Errorf("final status = %q, want %q", got.statusMsg, "Created: demo")
	}

	created := filepath.Join(parent, "demo")
	if _, err := os.Stat(filepath.Join(created, "config", ".pmproject")); err != nil {
		t.Errorf("project config not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(created, "main", "main.cc")); err != nil {
		t.Errorf("default main.cc not created: %v", err)
	}
	if s.Size() != 1 || s.ActiveProjectPath() != created {
		t.Errorf("solution size=%d active=%q, want 1 / %q",
			s.Size(), s.ActiveProjectPath(), created)
	}
	if !strings.Contains(out.String(), "Project Manager") {
		t.Errorf("output does not contain rendered UI")
	}
}

// TestProgramInstallDependencyEndToEnd 通过真实事件循环覆盖
// 「配置页 → Dependencies → 选择包 → 安装」这条链路。
func TestProgramInstallDependencyEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping program end-to-end test in short mode")
	}

	m, active := newTestModelWithPkg(t)
	m.ready = true
	m.width, m.height = 100, 32
	m.page = PageConfig
	m.cursor = configRowDependencies

	var out bytes.Buffer
	p := tea.NewProgram(m, tea.WithInput(strings.NewReader("")), tea.WithOutput(&out))

	key := func(t tea.KeyType) tea.Msg { return tea.KeyMsg{Type: t} }
	runes := func(v string) tea.Msg {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(v)}
	}

	script := []tea.Msg{
		key(tea.KeyEnter), // → 依赖列表页
		runes("a"),        // → 本地仓库安装页
		key(tea.KeyEnter), // 安装选中的包
	}

	go func() {
		for _, msg := range script {
			p.Send(msg)
		}
		p.Quit()
	}()

	final, err := p.Run()
	if err != nil {
		t.Fatalf("program run: %v", err)
	}

	got, ok := final.(MainModel)
	if !ok {
		t.Fatalf("final model type = %T, want MainModel", final)
	}
	if got.page != PageDepsList {
		t.Errorf("final page = %v, want PageDepsList", got.page)
	}
	if got.statusMsg != "Installed mylib@1.0.0" {
		t.Errorf("final status = %q, want %q", got.statusMsg, "Installed mylib@1.0.0")
	}

	deps := active.Config().Dependencies
	if len(deps) != 1 || deps[0].Name != "mylib" || deps[0].Version != "1.0.0" {
		t.Errorf("dependencies = %+v, want mylib@1.0.0", deps)
	}

	rendered := out.String()
	for _, want := range []string{"Dependencies", "mylib", "installed"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered output missing %q", want)
		}
	}
}
