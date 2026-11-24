package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rjf12138/project_manager/core"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00ADD8")).Padding(0, 1)
	activeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#00ADD8")).Bold(true).Padding(0, 1)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")).Bold(true)
	normalStyle   = lipgloss.NewStyle().Padding(0, 1)
)

type Page int

const (
	PageMainMenu Page = iota
	PageCreateProject
	PageLoadProject
	PageBuild
	PageBuildLog
	PageConfig
	PageList
	PageDepsEdit
	PageLibDirsEdit
	PageIncDirsEdit
	PageSrcDirsEdit
	PageDepsList    // 已安装依赖列表
	PagePkgList     // 本地仓库包列表（用于安装，默认按包名聚合）
	PagePkgVersions // 某个包的版本列表
)

// 配置页各行下标（与 renderConfigPage 的行顺序一一对应）
const (
	configRowName = iota
	configRowVersion
	configRowPath
	configRowBuildMode
	configRowGenerateType
	configRowCXXStandard
	configRowMainFile
	configRowDependencies
	configRowLibs
	configRowLibDirs
	configRowIncDirs
	configRowSrcDirs
	configRowCount
)

type MainModel struct {
	solution *core.Solution
	pm       *core.PackageManager
	page     Page
	ready    bool

	width  int
	height int

	directMode bool // 直接模式：-l/-cfg 进入时 q/esc 直接退出

	spinner spinner.Model
	loading bool
	loadMsg string

	inputs     []textinput.Model
	loadInput  textinput.Model
	focusIndex int

	cursor int // 用于主菜单和各页面的上下选择

	statusMsg  string
	statusType string

	editingCfg    core.ProjectConfig
	cfgModified   bool
	cfgEditMode   string // "libs", "lib_dirs", "inc_dirs", "src_dirs"
	cfgEditCursor int
	cfgEditFocus  int
	cfgEditValue  string // 直接存储输入值，不用 textinput
	cfgEditRunes  []rune // 输入模式的字符缓冲，保证多字节字符可正确删除

	buildLog  []string
	logOffset int

	depsCursor      int                   // 依赖列表游标
	pkgCursor       int                   // 仓库包列表游标（主页面为包，版本页为版本）
	repoSummaries   []core.PackageSummary // 按包名聚合的仓库视图
	repoPkgs        []core.PackageInfo    // 当前展开/展开过的版本清单
	pkgVersionOwner string                // 版本页对应的包名
	pkgParentCursor int                   // 从包列表进入版本页前的游标

	// openInEditorPath 记录退出 TUI 后要用编辑器打开的项目路径（pm -l 回车选择时设置）
	openInEditorPath string
}

type buildDoneMsg struct {
	err error
	log string
}

type runDoneMsg struct{ err error }

func NewMainModel(s *core.Solution) MainModel {
	inputs := make([]textinput.Model, 2)
	for i := range inputs {
		t := textinput.New()
		t.CharLimit = 128
		switch i {
		case 0:
			t.Placeholder = "Project name"
			t.Focus()
		case 1:
			t.Placeholder = "Parent directory (default: .)"
			t.SetValue(".")
		}
		inputs[i] = t
	}

	sr := spinner.New()
	sr.Spinner = spinner.Dot
	sr.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ADD8"))

	li := textinput.New()
	li.CharLimit = 256
	li.Placeholder = "Project path (e.g. ~/work/myapp)"

	pm := core.NewPackageManager()
	_ = pm.Init() // 失败时不阻塞界面，实际操作时会再次报错
	if dir, err := core.MaterializeBuiltin(); err == nil && dir != "" {
		pm.SetBuiltinDir(dir)
	}

	return MainModel{
		solution:   s,
		pm:         pm,
		page:       PageMainMenu,
		spinner:    sr,
		inputs:     inputs,
		loadInput:  li,
		focusIndex: 0,
	}
}

func (m MainModel) Init() tea.Cmd {
	return nil
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ready = true
		m.width = msg.Width
		m.height = msg.Height
		if w := msg.Width - 16; w > 10 {
			m.loadInput.Width = w
			for i := range m.inputs {
				m.inputs[i].Width = w
			}
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case buildDoneMsg:
		m.loading = false
		m.page = PageBuildLog
		m.buildLog = strings.Split(strings.TrimRight(msg.log, "\n"), "\n")
		if len(m.buildLog) == 1 && m.buildLog[0] == "" {
			m.buildLog = nil
		}
		m.logOffset = 0
		if msg.err != nil {
			m.statusMsg = "Build failed: " + msg.err.Error()
			m.statusType = "error"
			m.gotoLogTail()
		} else {
			m.statusMsg = "Build completed!"
			m.statusType = "success"
			m.gotoLogTail()
		}
		return m, nil

	case runDoneMsg:
		if msg.err != nil {
			m.statusMsg = "Run finished with error: " + msg.err.Error()
			m.statusType = "error"
		} else {
			m.statusMsg = "Run finished"
			m.statusType = "success"
		}
		m.page = PageMainMenu
		return m, nil
	}

	return m, nil
}

func (m MainModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// 全局：ctrl+c 永远可退出（包括正在输入内容时）
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	// 列表项的内联输入模式
	if m.cfgEditFocus == 1 {
		return m.handleCfgEditInput(msg)
	}

	// 页面级的输入控件优先处理
	switch m.page {
	case PageCreateProject, PageLoadProject:
		return m.handleFormKey(msg)
	case PageDepsEdit, PageLibDirsEdit, PageIncDirsEdit, PageSrcDirsEdit:
		return m.handleCfgEditKey(msg)
	case PageDepsList:
		return m.handleDepsListKey(key)
	case PagePkgList:
		return m.handlePkgListKey(key)
	case PagePkgVersions:
		return m.handlePkgVersionsKey(key)
	case PageBuildLog:
		return m.handleBuildLogKey(key)
	}

	// 主菜单按 q 直接退出
	if key == "q" && m.page == PageMainMenu {
		return m, tea.Quit
	}

	// 其他页面 q / esc 返回上一层；直接模式下直接退出
	if key == "q" || key == "esc" {
		if m.directMode {
			return m, tea.Quit
		}
		if m.page != PageMainMenu {
			m.page = PageMainMenu
			m.statusMsg = ""
			m.statusType = ""
			m.cursor = 0
		}
		return m, nil
	}

	// 全局：上下键移动光标
	if key == "up" {
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	}
	if key == "down" {
		maxLen := m.getMenuLength()
		if m.cursor < maxLen-1 {
			m.cursor++
		}
		return m, nil
	}

	// 全局：回车触发操作
	if key == "enter" {
		return m.handleEnter()
	}

	if m.page == PageConfig {
		return m.handleConfigShortcut(key)
	}
	if m.page == PageBuild {
		return m.handleBuildShortcut(key)
	}

	return m, nil
}

// ===================== 表单输入页（新建 / 加载项目） =====================

func (m MainModel) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if key == "esc" {
		m.page = PageMainMenu
		m.statusMsg = ""
		m.statusType = ""
		return m, nil
	}

	if m.page == PageLoadProject {
		if key == "enter" {
			return m.submitLoad()
		}
		var cmd tea.Cmd
		m.loadInput, cmd = m.loadInput.Update(msg)
		return m, cmd
	}

	// PageCreateProject
	if key == "up" || key == "down" || key == "tab" || key == "shift+tab" {
		if key == "up" || key == "shift+tab" {
			m.focusIndex--
		} else {
			m.focusIndex++
		}
		if m.focusIndex < 0 {
			m.focusIndex = 0
		}
		if m.focusIndex > 1 {
			m.focusIndex = 1
		}
		for i := 0; i < 2; i++ {
			if i == m.focusIndex {
				m.inputs[i].Focus()
			} else {
				m.inputs[i].Blur()
			}
		}
		return m, nil
	}
	if key == "enter" {
		return m.submitCreate()
	}
	// 其他按键传递给 textinput
	var cmds []tea.Cmd
	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m MainModel) submitCreate() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.inputs[0].Value())
	parentPath := strings.TrimSpace(m.inputs[1].Value())
	if name == "" {
		m.statusMsg = "Project name required"
		m.statusType = "error"
		return m, nil
	}
	if parentPath == "" {
		parentPath = "."
	}

	fullPath, err := core.CreateProject(name, parentPath)
	if err != nil {
		m.statusMsg = "Failed: " + err.Error()
		m.statusType = "error"
		return m, nil
	}
	if err := m.solution.LoadProject(fullPath); err != nil {
		m.statusMsg = "Created but failed to load: " + err.Error()
		m.statusType = "error"
		return m, nil
	}

	m.inputs[0].SetValue("")
	m.inputs[1].SetValue(".")
	m.focusIndex = 0
	m.inputs[0].Focus()
	m.inputs[1].Blur()
	m.statusMsg = "Created: " + name
	m.statusType = "success"
	m.page = PageMainMenu
	m.cursor = 0
	return m, nil
}

// isPMProjectDir 判断目录是否为 pm 项目：
// 当前格式为 <dir>/.pmconfig/config.json；同时兼容早期单文件 .pmconfig
// 与旧版 <dir>/config/.pmproject。
func isPMProjectDir(dir string) bool {
	candidates := []string{
		filepath.Join(dir, core.ConfigDirName, core.ConfigFileName),
		filepath.Join(dir, core.ConfigDirName), // 早期单文件格式
		filepath.Join(dir, "config", core.LegacyConfigFileName),
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return true
		}
	}
	return false
}

func (m MainModel) submitLoad() (tea.Model, tea.Cmd) {
	raw := strings.TrimSpace(m.loadInput.Value())
	if raw == "" {
		m.statusMsg = "Project path required"
		m.statusType = "error"
		return m, nil
	}

	if raw == "~" || strings.HasPrefix(raw, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if raw == "~" {
				raw = home
			} else {
				raw = filepath.Join(home, raw[2:])
			}
		}
	}

	abs, err := filepath.Abs(raw)
	if err != nil {
		m.statusMsg = "Failed: " + err.Error()
		m.statusType = "error"
		return m, nil
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		m.statusMsg = "Directory not found: " + abs
		m.statusType = "error"
		return m, nil
	}
	if !isPMProjectDir(abs) {
		m.statusMsg = "Not a pm project (.pmconfig/config.json missing): " + abs
		m.statusType = "error"
		return m, nil
	}
	if err := m.solution.LoadProject(abs); err != nil {
		m.statusMsg = "Failed: " + err.Error()
		m.statusType = "error"
		return m, nil
	}

	m.loadInput.SetValue("")
	m.statusMsg = "Loaded: " + m.solution.ProjectNameFromPath(abs)
	m.statusType = "success"
	m.page = PageMainMenu
	m.cursor = 0
	return m, nil
}

// ===================== 构建日志页 =====================

// logPageSize 日志页可见行数
func (m MainModel) logPageSize() int {
	h := 18
	if m.height > 0 {
		h = m.height - 12
		if h < 5 {
			h = 5
		}
		if h > 60 {
			h = 60
		}
	}
	return h
}

func (m MainModel) maxLogOffset() int {
	if len(m.buildLog) <= m.logPageSize() {
		return 0
	}
	return len(m.buildLog) - m.logPageSize()
}

func (m *MainModel) gotoLogTail() {
	m.logOffset = m.maxLogOffset()
}

func (m MainModel) handleBuildLogKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.logOffset > 0 {
			m.logOffset--
		}
	case "down", "j":
		if m.logOffset < m.maxLogOffset() {
			m.logOffset++
		}
	case "pgup":
		m.logOffset -= m.logPageSize() / 2
		if m.logOffset < 0 {
			m.logOffset = 0
		}
	case "pgdown":
		m.logOffset += m.logPageSize() / 2
		if m.logOffset > m.maxLogOffset() {
			m.logOffset = m.maxLogOffset()
		}
	case "home", "g":
		m.logOffset = 0
	case "end", "G":
		m.gotoLogTail()
	case "esc", "q":
		if m.directMode {
			return m, tea.Quit
		}
		m.page = PageBuild
		m.cursor = 0
	}
	return m, nil
}

func (m MainModel) getMenuLength() int {
	switch m.page {
	case PageMainMenu:
		n := 2 // load + create
		if m.solution.Size() > 0 {
			n += 6 // build, rebuild, run, config, clean, list
		}
		n += 1 // quit
		return n
	case PageConfig:
		return int(configRowCount)
	case PageBuild:
		return 4
	case PageList:
		return m.solution.Size()
	default:
		return 1
	}
}

func (m MainModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.page {
	case PageMainMenu:
		return m.activateMainMenuItem()
	case PageBuild:
		return m.activateBuildItem()
	case PageList:
		return m.activateListItem()
	case PageConfig:
		return m.handleConfigEnter()
	default:
		return m, nil
	}
}

// ===================== Main Menu =====================

func (m MainModel) activateMainMenuItem() (tea.Model, tea.Cmd) {
	idx := m.cursor
	if m.solution.Size() == 0 {
		switch idx {
		case 0:
			m.page = PageLoadProject
			m.loadInput.SetValue("")
			m.loadInput.Focus()
			m.statusMsg = ""
			m.statusType = ""
			return m, textinput.Blink
		case 1:
			m.page = PageCreateProject
			m.inputs[0].SetValue("")
			m.inputs[1].SetValue(".")
			m.focusIndex = 0
			m.inputs[0].Focus()
			m.inputs[1].Blur()
			return m, textinput.Blink
		case 2:
			return m, tea.Quit
		}
	} else {
		switch idx {
		case 0:
			m.page = PageLoadProject
			m.loadInput.SetValue("")
			m.loadInput.Focus()
			m.statusMsg = ""
			m.statusType = ""
			return m, textinput.Blink
		case 1:
			m.page = PageCreateProject
			m.inputs[0].SetValue("")
			m.inputs[1].SetValue(".")
			m.focusIndex = 0
			m.inputs[0].Focus()
			m.inputs[1].Blur()
			return m, textinput.Blink
		case 2:
			active := m.solution.ActiveProject()
			if active != nil {
				m.page = PageBuild
				m.cursor = 0
			}
		case 3:
			active := m.solution.ActiveProject()
			if active != nil {
				return m.startBuild(active, true, "Rebuilding...")
			}
		case 4:
			active := m.solution.ActiveProject()
			if active != nil {
				return m.startRun(active)
			}
		case 5:
			active := m.solution.ActiveProject()
			if active != nil {
				m.page = PageConfig
				m.editingCfg = active.Config()
				m.cfgModified = false
				m.cursor = 0
			}
		case 6:
			active := m.solution.ActiveProject()
			if active != nil {
				if err := active.Clean(); err != nil {
					m.statusMsg = "Clean failed: " + err.Error()
					m.statusType = "error"
				} else {
					m.statusMsg = "Clean completed"
					m.statusType = "success"
				}
			}
		case 7:
			m.page = PageList
			m.cursor = 0
		case 8:
			return m, tea.Quit
		}
	}
	return m, nil
}

// startBuild 启动异步构建，同时驱动 spinner 动画
func (m MainModel) startBuild(proj *core.Project, rebuild bool, msg string) (tea.Model, tea.Cmd) {
	m.loading = true
	m.loadMsg = msg
	return m, tea.Batch(m.runBuild(proj, rebuild), m.spinner.Tick)
}

// startRun 把终端交给被运行的程序（tea.ExecProcess），退出后自动恢复界面
func (m MainModel) startRun(proj *core.Project) (tea.Model, tea.Cmd) {
	cmd, err := proj.RunCommand()
	if err != nil {
		m.statusMsg = err.Error()
		m.statusType = "error"
		return m, nil
	}
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return runDoneMsg{err: err}
	})
}

// ===================== Build =====================

func (m MainModel) activateBuildItem() (tea.Model, tea.Cmd) {
	active := m.solution.ActiveProject()
	if active == nil {
		m.page = PageMainMenu
		return m, nil
	}
	switch m.cursor {
	case 0:
		return m.startBuild(active, false, "Building...")
	case 1:
		return m.startBuild(active, true, "Rebuilding...")
	case 2:
		cfg := active.Config()
		cfg.BuildMode = core.BuildModeDebug
		if err := active.SaveConfig(cfg); err != nil {
			m.statusMsg = "Save failed: " + err.Error()
			m.statusType = "error"
			return m, nil
		}
		m.statusMsg = "Mode: debug"
		m.statusType = "info"
	case 3:
		cfg := active.Config()
		cfg.BuildMode = core.BuildModeRelease
		if err := active.SaveConfig(cfg); err != nil {
			m.statusMsg = "Save failed: " + err.Error()
			m.statusType = "error"
			return m, nil
		}
		m.statusMsg = "Mode: release"
		m.statusType = "info"
	}
	return m, nil
}

func (m MainModel) handleBuildShortcut(key string) (tea.Model, tea.Cmd) {
	active := m.solution.ActiveProject()
	if active == nil {
		m.page = PageMainMenu
		return m, nil
	}
	switch key {
	case "d":
		m.cursor = 2
		return m.activateBuildItem()
	case "r":
		m.cursor = 3
		return m.activateBuildItem()
	case "l":
		if len(m.buildLog) > 0 {
			m.page = PageBuildLog
			m.logOffset = 0
		} else {
			m.statusMsg = "No build log yet"
			m.statusType = "info"
		}
	}
	return m, nil
}

func (m MainModel) runBuild(proj *core.Project, rebuild bool) tea.Cmd {
	return func() tea.Msg {
		log, err := proj.BuildCapture(rebuild)
		return buildDoneMsg{err: err, log: log}
	}
}

// ===================== Project List =====================

func (m MainModel) activateListItem() (tea.Model, tea.Cmd) {
	paths := m.solution.ProjectPaths()
	if m.cursor >= len(paths) {
		return m, nil
	}
	selected := paths[m.cursor]
	m.solution.SetActiveProject(selected)
	if m.directMode {
		// pm -l 直接模式：切换活动项目后退出，并由 RunProjectList 用编辑器打开
		m.openInEditorPath = selected
		return m, tea.Quit
	}
	m.page = PageMainMenu
	m.cursor = 0
	m.statusMsg = "Switched to: " + m.solution.ProjectNameFromPath(selected)
	m.statusType = "success"
	return m, nil
}

func (m MainModel) handleConfigShortcut(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "b":
		m.editingCfg.BuildMode = toggleBuildMode(m.editingCfg.BuildMode)
		m.cfgModified = true
	case "g":
		m.editingCfg.GenerateType = cycleGenerateType(m.editingCfg.GenerateType)
		m.cfgModified = true
	case "f":
		active := m.solution.ActiveProject()
		if active != nil {
			files, _ := active.ScanMainFiles()
			found := false
			for i, f := range files {
				if f == m.editingCfg.MainFile {
					m.editingCfg.MainFile = files[(i+1)%len(files)]
					m.cfgModified = true
					found = true
					break
				}
			}
			if !found && len(files) > 0 {
				m.editingCfg.MainFile = files[0]
				m.cfgModified = true
			}
		}
	case "s":
		active := m.solution.ActiveProject()
		if active != nil && m.cfgModified {
			if err := active.SaveConfig(m.editingCfg); err != nil {
				m.statusMsg = "Save failed: " + err.Error()
				m.statusType = "error"
				return m, nil
			}
			m.statusMsg = "Config saved"
			m.statusType = "success"
			m.cfgModified = false
		}
		m.page = PageMainMenu
	case "c":
		m.page = PageMainMenu
		// 回车进入各项的编辑子页面
	case "enter":
		return m.handleConfigEnter()
	}
	return m, nil
}

func (m MainModel) handleConfigEnter() (tea.Model, tea.Cmd) {
	switch m.cursor {
	case configRowDependencies:
		return m.enterDepsList()
	case configRowLibs:
		m.page = PageDepsEdit
		m.cfgEditMode = "libs"
		m.cfgEditCursor = 0
		return m, nil
	case configRowLibDirs:
		m.page = PageLibDirsEdit
		m.cfgEditMode = "lib_dirs"
		m.cfgEditCursor = 0
		return m, nil
	case configRowIncDirs:
		m.page = PageIncDirsEdit
		m.cfgEditMode = "inc_dirs"
		m.cfgEditCursor = 0
		return m, nil
	case configRowSrcDirs:
		m.page = PageSrcDirsEdit
		m.cfgEditMode = "src_dirs"
		m.cfgEditCursor = 0
		return m, nil
	default:
		return m, nil
	}
}

// ===================== 依赖管理：依赖列表 / 安装包 =====================

// enterDepsList 进入依赖列表页
func (m MainModel) enterDepsList() (tea.Model, tea.Cmd) {
	m.page = PageDepsList
	m.depsCursor = 0
	m.statusMsg = ""
	m.statusType = ""
	return m, nil
}

// enterPkgList 加载本地仓库的包并进入安装页（默认按包名聚合）
func (m MainModel) enterPkgList() (tea.Model, tea.Cmd) {
	summaries, err := m.pm.ListPackageSummaries()
	if err != nil {
		m.statusMsg = "Read repository failed: " + err.Error()
		m.statusType = "error"
		return m, nil
	}

	m.repoSummaries = summaries
	m.repoPkgs = m.repoPkgs[:0]
	for _, s := range summaries {
		m.repoPkgs = append(m.repoPkgs, s.Versions...)
	}
	m.pkgCursor = 0
	m.pkgVersionOwner = ""
	m.statusMsg = ""
	m.statusType = ""
	m.page = PagePkgList
	return m, nil
}

// savePendingCfg 执行包操作前把配置页里未保存的改动落盘，避免随后被磁盘配置覆盖丢失
func (m *MainModel) savePendingCfg(active *core.Project) error {
	if !m.cfgModified {
		return nil
	}
	if err := active.SaveConfig(m.editingCfg); err != nil {
		return err
	}
	m.cfgModified = false
	return nil
}

// reloadCfg 以磁盘配置为准刷新编辑副本
func (m *MainModel) reloadCfg(active *core.Project) {
	m.editingCfg = active.Config()
	m.cfgModified = false
}

func (m MainModel) handleDepsListKey(key string) (tea.Model, tea.Cmd) {
	active := m.solution.ActiveProject()
	if active == nil {
		m.page = PageConfig
		return m, nil
	}
	deps := active.Config().Dependencies

	switch key {
	case "up":
		if m.depsCursor > 0 {
			m.depsCursor--
		}
	case "down":
		if m.depsCursor < len(deps)-1 {
			m.depsCursor++
		}
	case "a":
		return m.enterPkgList()
	case "d":
		if m.depsCursor < len(deps) {
			return m.uninstallDep(deps[m.depsCursor].Name)
		}
	case "r":
		if m.depsCursor < len(deps) {
			return m.reinstallDep(deps[m.depsCursor])
		}
	case "u":
		return m.reinstallAll()
	case "esc", "q":
		if m.directMode && key == "q" {
			return m, tea.Quit
		}
		m.page = PageConfig
		m.cursor = configRowDependencies
	}
	return m, nil
}

func (m MainModel) handlePkgListKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up":
		if m.pkgCursor > 0 {
			m.pkgCursor--
		}
	case "down":
		if m.pkgCursor < len(m.repoSummaries)-1 {
			m.pkgCursor++
		}
	case "enter":
		if m.pkgCursor < len(m.repoSummaries) {
			return m.installPackage(m.repoSummaries[m.pkgCursor].Latest)
		}
	case "v", "right", "l":
		return m.openPkgVersions()
	case "esc", "q":
		if m.directMode && key == "q" {
			return m, tea.Quit
		}
		m.page = PageDepsList
	}
	return m, nil
}

// openPkgVersions 进入选中包的版本列表页
func (m MainModel) openPkgVersions() (tea.Model, tea.Cmd) {
	if m.pkgCursor >= len(m.repoSummaries) {
		return m, nil
	}
	summary := m.repoSummaries[m.pkgCursor]
	m.repoPkgs = append(m.repoPkgs[:0], summary.Versions...)
	m.pkgVersionOwner = summary.Name
	m.pkgParentCursor = m.pkgCursor
	m.pkgCursor = 0
	m.page = PagePkgVersions
	return m, nil
}

func (m MainModel) handlePkgVersionsKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up":
		if m.pkgCursor > 0 {
			m.pkgCursor--
		}
	case "down":
		if m.pkgCursor < len(m.repoPkgs)-1 {
			m.pkgCursor++
		}
	case "enter":
		if m.pkgCursor < len(m.repoPkgs) {
			return m.installPackage(m.repoPkgs[m.pkgCursor])
		}
	case "esc", "q":
		if m.directMode && key == "q" {
			return m, tea.Quit
		}
		m.page = PagePkgList
		m.pkgCursor = m.pkgParentCursor
	}
	return m, nil
}

// installPackage 把仓库中的包安装到活动项目（自动写入配置）
func (m MainModel) installPackage(pkg core.PackageInfo) (tea.Model, tea.Cmd) {
	active := m.solution.ActiveProject()
	if active == nil {
		m.statusMsg = "No active project"
		m.statusType = "error"
		return m, nil
	}
	if err := (&m).savePendingCfg(active); err != nil {
		m.statusMsg = "Save failed: " + err.Error()
		m.statusType = "error"
		return m, nil
	}

	dep := core.Dependency{Name: pkg.Manifest.Name, Version: pkg.Manifest.Version}
	if _, err := m.pm.InstallToProject(active, dep); err != nil {
		m.statusMsg = "Install failed: " + err.Error()
		m.statusType = "error"
		return m, nil
	}

	(&m).reloadCfg(active)
	m.depsCursor = indexOfDependency(active.Config().Dependencies, dep.Name)
	m.statusMsg = fmt.Sprintf("Installed %s@%s", dep.Name, dep.Version)
	m.statusType = "success"
	m.page = PageDepsList
	return m, nil
}

// uninstallDep 卸载依赖，并清理已安装的文件
func (m MainModel) uninstallDep(name string) (tea.Model, tea.Cmd) {
	active := m.solution.ActiveProject()
	if active == nil {
		m.page = PageConfig
		return m, nil
	}
	if err := (&m).savePendingCfg(active); err != nil {
		m.statusMsg = "Save failed: " + err.Error()
		m.statusType = "error"
		return m, nil
	}

	removed, err := active.UninstallDependency(name)
	if err != nil {
		m.statusMsg = "Remove failed: " + err.Error()
		m.statusType = "error"
		return m, nil
	}

	(&m).reloadCfg(active)
	if n := len(active.Config().Dependencies); m.depsCursor >= n {
		m.depsCursor = n - 1
		if m.depsCursor < 0 {
			m.depsCursor = 0
		}
	}
	m.statusMsg = fmt.Sprintf("Removed %s (%d file(s) cleaned)", name, len(removed))
	m.statusType = "success"
	return m, nil
}

// reinstallDep 重新安装指定依赖（修复文件缺失或按配置换回指定版本）
func (m MainModel) reinstallDep(dep core.Dependency) (tea.Model, tea.Cmd) {
	active := m.solution.ActiveProject()
	if active == nil {
		m.page = PageConfig
		return m, nil
	}
	if err := (&m).savePendingCfg(active); err != nil {
		m.statusMsg = "Save failed: " + err.Error()
		m.statusType = "error"
		return m, nil
	}

	if _, err := m.pm.InstallToProject(active, dep); err != nil {
		m.statusMsg = "Reinstall failed: " + err.Error()
		m.statusType = "error"
		return m, nil
	}

	(&m).reloadCfg(active)
	m.statusMsg = fmt.Sprintf("Reinstalled %s@%s", dep.Name, dep.Version)
	m.statusType = "success"
	return m, nil
}

// reinstallAll 重新安装配置中声明的全部依赖
func (m MainModel) reinstallAll() (tea.Model, tea.Cmd) {
	active := m.solution.ActiveProject()
	if active == nil {
		m.page = PageConfig
		return m, nil
	}
	if err := (&m).savePendingCfg(active); err != nil {
		m.statusMsg = "Save failed: " + err.Error()
		m.statusType = "error"
		return m, nil
	}

	manifests, err := m.pm.InstallProjectDeps(active)
	if err != nil {
		m.statusMsg = "Reinstall failed: " + err.Error()
		m.statusType = "error"
		return m, nil
	}

	(&m).reloadCfg(active)
	m.statusMsg = fmt.Sprintf("Reinstalled %d package(s)", len(manifests))
	m.statusType = "success"
	return m, nil
}

// indexOfDependency 返回依赖在列表中的下标，找不到返回 0
func indexOfDependency(deps []core.Dependency, name string) int {
	for i, d := range deps {
		if d.Name == name {
			return i
		}
	}
	return 0
}

// ===================== 配置子页面：编辑列表项 =====================

func (m MainModel) getEditList() []string {
	switch m.cfgEditMode {
	case "libs":
		return m.editingCfg.LegacyDeps.Libs
	case "lib_dirs":
		return m.editingCfg.LegacyDeps.LibDirs
	case "inc_dirs":
		return m.editingCfg.LegacyDeps.IncDirs
	case "src_dirs":
		return m.editingCfg.SourceDirs
	}
	return nil
}

// setEditList 写回当前编辑列表。必须使用指针接收者，否则修改会丢失。
func (m *MainModel) setEditList(items []string) {
	switch m.cfgEditMode {
	case "libs":
		m.editingCfg.LegacyDeps.Libs = items
	case "lib_dirs":
		m.editingCfg.LegacyDeps.LibDirs = items
	case "inc_dirs":
		m.editingCfg.LegacyDeps.IncDirs = items
	case "src_dirs":
		m.editingCfg.SourceDirs = items
	}
	m.cfgModified = true
}

// handleCfgEditInput 处理列表项内联输入（自己维护字符缓冲，支持多字节字符）
func (m MainModel) handleCfgEditInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		val := strings.TrimSpace(string(m.cfgEditRunes))
		m.cfgEditRunes = nil
		m.cfgEditValue = ""
		m.cfgEditFocus = 0
		if val == "" {
			m.statusMsg = "Empty input, item not saved"
			m.statusType = "error"
			return m, nil
		}
		(&m).applyEditValue(val)
		m.statusMsg = "Item saved: " + val
		m.statusType = "success"
	case "esc":
		m.cfgEditRunes = nil
		m.cfgEditValue = ""
		m.cfgEditFocus = 0
		m.statusMsg = "Edit cancelled"
		m.statusType = "info"
	case "backspace", "ctrl+h":
		if len(m.cfgEditRunes) > 0 {
			m.cfgEditRunes = m.cfgEditRunes[:len(m.cfgEditRunes)-1]
		}
	case "ctrl+u":
		m.cfgEditRunes = nil
	default:
		// 过滤控制键，只接受可打印字符
		if len(msg.Runes) > 0 && !msg.Alt {
			m.cfgEditRunes = append(m.cfgEditRunes, msg.Runes...)
		}
	}

	m.cfgEditValue = string(m.cfgEditRunes)
	return m, nil
}

// applyEditValue 把输入值写入当前编辑列表（越界则追加）
func (m *MainModel) applyEditValue(val string) {
	items := m.getEditList()
	if m.cfgEditCursor >= len(items) {
		items = append(items, val)
		m.cfgEditCursor = len(items) - 1
	} else {
		items[m.cfgEditCursor] = val
	}
	m.setEditList(items)
}

func (m MainModel) handleCfgEditKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	items := m.getEditList()
	switch key {
	case "up":
		if m.cfgEditCursor > 0 {
			m.cfgEditCursor--
		}
	case "down":
		if m.cfgEditCursor < len(items) {
			m.cfgEditCursor++
		}
	case "a":
		m.cfgEditCursor = len(items)
		m.cfgEditRunes = nil
		m.cfgEditValue = ""
		m.cfgEditFocus = 1
	case "enter":
		m.cfgEditValue = ""
		m.cfgEditRunes = nil
		if m.cfgEditCursor < len(items) {
			m.cfgEditRunes = []rune(items[m.cfgEditCursor])
			m.cfgEditValue = items[m.cfgEditCursor]
		}
		m.cfgEditFocus = 1
	case "d":
		if m.cfgEditCursor < len(items) {
			m.setEditList(append(append([]string{}, items[:m.cfgEditCursor]...), items[m.cfgEditCursor+1:]...))
			remaining := m.getEditList()
			if m.cfgEditCursor >= len(remaining) && len(remaining) > 0 {
				m.cfgEditCursor = len(remaining) - 1
			}
			m.statusMsg = "Item deleted"
			m.statusType = "info"
		}
	case "esc", "q":
		if m.directMode && key == "q" {
			return m, tea.Quit
		}
		m.page = PageConfig
		m.cursor = 0
		return m, nil
	}
	return m, nil
}

func toggleBuildMode(m core.BuildMode) core.BuildMode {
	if m == core.BuildModeDebug {
		return core.BuildModeRelease
	}
	return core.BuildModeDebug
}

func cycleGenerateType(t core.GenerateType) core.GenerateType {
	types := []core.GenerateType{
		core.GenerateTypeExecutable,
		core.GenerateTypeStaticLib,
		core.GenerateTypeSharedLib,
	}
	for i, tt := range types {
		if tt == t {
			return types[(i+1)%len(types)]
		}
	}
	return core.GenerateTypeExecutable
}

// ===================== View =====================

func (m MainModel) View() string {
	if !m.ready {
		return "Loading..."
	}
	if m.loading {
		return lipgloss.NewStyle().Padding(2).Render(
			titleStyle.Render("Project Manager") + "\n\n" +
				m.spinner.View() + " " + m.loadMsg)
	}

	var content string
	switch m.page {
	case PageCreateProject:
		content = m.renderCreateProject()
	case PageLoadProject:
		content = m.renderLoadProject()
	case PageBuild:
		content = m.renderBuildPage()
	case PageBuildLog:
		content = m.renderBuildLogPage()
	case PageConfig:
		content = m.renderConfigPage()
	case PageDepsEdit, PageLibDirsEdit, PageIncDirsEdit, PageSrcDirsEdit:
		content = m.renderCfgEditPage()
	case PageDepsList:
		content = m.renderDepsList()
	case PagePkgList:
		content = m.renderPkgList()
	case PagePkgVersions:
		content = m.renderPkgVersions()
	case PageList:
		content = m.renderListPage()
	default:
		content = m.renderMainMenu()
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

// ===================== 渲染函数 =====================

// statusStyle 按状态类型返回样式，info 不再被误显示为错误
func statusStyle(t string) lipgloss.Style {
	switch t {
	case "error":
		return errorStyle
	case "success":
		return successStyle
	case "info":
		return dimStyle
	default:
		return helpStyle
	}
}

// renderStatus 渲染状态行（为空时返回空串）
func (m MainModel) renderStatus() string {
	if m.statusMsg == "" {
		return ""
	}
	return "\n" + statusStyle(m.statusType).Render("• "+m.statusMsg)
}

func (m MainModel) renderMainMenu() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Project Manager") + "\n\n")

	active := m.solution.ActiveProject()
	if active != nil {
		cfg := active.Config()
		sb.WriteString(activeStyle.Render(fmt.Sprintf("◆ Active: %s  [%s]", cfg.Name, cfg.BuildMode)) + "\n")
		sb.WriteString(fmt.Sprintf("  Path:     %s\n", cfg.Path))
		sb.WriteString(fmt.Sprintf("  Version:  %s  |  C++%d  |  %s\n", cfg.Version, cfg.CXXStandard, cfg.GenerateType))
		sb.WriteString(fmt.Sprintf("  Main:     %s\n", cfg.MainFile))
		sb.WriteString(fmt.Sprintf("  Deps: %d  |  Targets: %d\n\n", len(cfg.Dependencies), len(cfg.Targets)))
	} else {
		sb.WriteString(dimStyle.Render("◇ No active project") + "\n\n")
	}
	sb.WriteString(fmt.Sprintf("Loaded projects: %d\n\n", m.solution.Size()))

	items := []string{
		"Load / Open Project",
		"Create New Project",
	}
	if m.solution.Size() > 0 {
		items = append(items,
			"Build (Incremental)",
			"Rebuild (Clean + Build)",
			"Run",
			"Configure",
			"Clean Artifacts",
			"List Projects",
		)
	}
	items = append(items, "Quit")

	for i, it := range items {
		if i == m.cursor {
			sb.WriteString(cursorStyle.Render(fmt.Sprintf(" ▶ %s", it)) + "\n")
		} else {
			sb.WriteString(normalStyle.Render(fmt.Sprintf("   %s", it)) + "\n")
		}
	}

	if m.statusMsg != "" {
		sb.WriteString(m.renderStatus())
	}

	sb.WriteString("\n" + helpStyle.Render("↑↓ move • enter select • q quit"))
	return sb.String()
}

func (m MainModel) renderCreateProject() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Create Project") + "\n\n")
	sb.WriteString(fmt.Sprintf("  Name:    %s\n", m.inputs[0].View()))
	sb.WriteString(fmt.Sprintf("  Parent:  %s\n", m.inputs[1].View()))
	sb.WriteString("\n" + helpStyle.Render("↑↓ switch field • enter create • esc back"))
	sb.WriteString(m.renderStatus())
	return sb.String()
}

func (m MainModel) renderLoadProject() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Load Project") + "\n\n")
	sb.WriteString("  Path:  " + m.loadInput.View() + "\n")
	sb.WriteString("\n" + helpStyle.Render("enter load • esc back  (directory containing .pmconfig/config.json)"))
	sb.WriteString(m.renderStatus())
	return sb.String()
}

func (m MainModel) renderBuildPage() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Build") + "\n\n")

	active := m.solution.ActiveProject()
	if active == nil {
		return "No active project."
	}
	cfg := active.Config()
	sb.WriteString(fmt.Sprintf("Project: %s  |  Mode: %s\n\n", cfg.Name, cfg.BuildMode))

	items := []string{
		"Build (incremental)",
		"Rebuild (clean + build)",
		"Switch to Debug mode",
		"Switch to Release mode",
	}
	for i, it := range items {
		if i == m.cursor {
			sb.WriteString(cursorStyle.Render(fmt.Sprintf(" ▶ %s", it)) + "\n")
		} else {
			sb.WriteString(normalStyle.Render(fmt.Sprintf("   %s", it)) + "\n")
		}
	}

	hint := "\n↑↓ move • enter select • esc back"
	if len(m.buildLog) > 0 {
		hint += " • l view last log"
	}
	sb.WriteString(helpStyle.Render(hint))
	sb.WriteString(m.renderStatus())
	return sb.String()
}

func (m MainModel) renderBuildLogPage() string {
	var sb strings.Builder
	status := "Build succeeded"
	if m.statusType == "error" {
		status = "Build failed"
	}
	sb.WriteString(titleStyle.Render("Build Log") + "  " +
		statusStyle(m.statusType).Render(status) + "\n\n")

	if len(m.buildLog) == 0 {
		sb.WriteString(dimStyle.Render("  (no output)\n"))
	}

	pageSize := m.logPageSize()
	start := m.logOffset
	if start > m.maxLogOffset() {
		start = m.maxLogOffset()
	}
	end := start + pageSize
	if end > len(m.buildLog) {
		end = len(m.buildLog)
	}

	for _, line := range m.buildLog[start:end] {
		style := normalStyle
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "错误") {
			style = errorStyle
		}
		sb.WriteString(style.Render("  "+line) + "\n")
	}

	if len(m.buildLog) > pageSize {
		sb.WriteString(dimStyle.Render(fmt.Sprintf("\n  lines %d-%d / %d",
			start+1, end, len(m.buildLog))))
	}
	sb.WriteString(m.renderStatus())
	sb.WriteString("\n" + helpStyle.Render("↑↓/jk scroll • pgup/pgdn • g/G top/bottom • esc back"))
	return sb.String()
}

func (m MainModel) renderConfigPage() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Configure") + "\n\n")

	cfg := m.editingCfg
	lines := []string{
		fmt.Sprintf("Name:          %s", cfg.Name),
		fmt.Sprintf("Version:       %s", cfg.Version),
		fmt.Sprintf("Path:          %s", cfg.Path),
		fmt.Sprintf("Build Mode:    %s  [b]", cfg.BuildMode),
		fmt.Sprintf("Generate Type: %s  [g]", cfg.GenerateType),
		fmt.Sprintf("C++ Standard:  C++%d", cfg.CXXStandard),
		fmt.Sprintf("Main File:     %s  [f]", cfg.MainFile),
		fmt.Sprintf("Dependencies:  %d installed  [enter]", len(cfg.Dependencies)),
		fmt.Sprintf("Libraries:     %v  [enter]", cfg.LegacyDeps.Libs),
		fmt.Sprintf("Lib Dirs:      %v  [enter]", cfg.LegacyDeps.LibDirs),
		fmt.Sprintf("Include Dirs:  %v  [enter]", cfg.LegacyDeps.IncDirs),
		fmt.Sprintf("Source Dirs:   %v  [enter]", cfg.SourceDirs),
	}
	for i, line := range lines {
		if i == m.cursor {
			sb.WriteString(cursorStyle.Render(fmt.Sprintf(" ▶ %s", line)) + "\n")
		} else {
			sb.WriteString(normalStyle.Render(fmt.Sprintf("   %s", line)) + "\n")
		}
	}

	sb.WriteString(fmt.Sprintf("\n\n  [s] Save & Exit  |  [c] Cancel\n"))
	if m.cfgModified {
		sb.WriteString(dimStyle.Render("  * Modified\n"))
	}
	sb.WriteString(dimStyle.Render("  Libraries / Lib Dirs / Include Dirs are filled automatically by 'pm pkg install'"))
	if m.directMode {
		sb.WriteString(helpStyle.Render("\n↑↓ move cursor • b/g/f/s/c shortcuts • q quit"))
	} else {
		sb.WriteString(helpStyle.Render("\n↑↓ move cursor • b/g/f/s/c shortcuts • esc cancel"))
	}
	sb.WriteString(m.renderStatus())
	return sb.String()
}

func (m MainModel) renderListPage() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Projects") + "\n\n")

	paths := m.solution.ProjectPaths()
	if len(paths) == 0 {
		sb.WriteString("No projects loaded.\n")
		return sb.String()
	}

	active := m.solution.ActiveProject()
	activePath := ""
	if active != nil {
		activePath = active.Config().Path
	}

	for i, p := range paths {
		name := m.solution.ProjectNameFromPath(p)
		prefix := "  "
		if p == activePath {
			prefix = "◆ "
		}

		line := fmt.Sprintf("%s%s", prefix, name)
		if i == m.cursor {
			line = cursorStyle.Render(fmt.Sprintf(" ▶ %s", line))
		} else if p == activePath {
			line = activeStyle.Render(fmt.Sprintf("   %s", line))
		} else {
			line = normalStyle.Render(fmt.Sprintf("   %s", line))
		}
		sb.WriteString(line + "\n")
		sb.WriteString(dimStyle.Render(fmt.Sprintf("     %s", p)) + "\n\n")
	}

	if m.directMode {
		sb.WriteString(helpStyle.Render("↑↓ move • enter open in VS Code • q quit"))
		sb.WriteString("\n" + helpStyle.Render("enter switches the active project and opens it in VS Code"))
	} else {
		sb.WriteString(helpStyle.Render("↑↓ move • enter select • esc back"))
		sb.WriteString("\n" + helpStyle.Render("select to switch active project"))
	}
	return sb.String()
}

// renderDepsList 依赖列表页：显示已安装的包、版本与安装状态
func (m MainModel) renderDepsList() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Dependencies") + "\n\n")

	active := m.solution.ActiveProject()
	if active == nil {
		return "No active project."
	}
	cfg := active.Config()
	sb.WriteString(fmt.Sprintf("Project: %s   (lib/%s)\n\n", cfg.Name, cfg.BuildMode))

	if len(cfg.Dependencies) == 0 {
		sb.WriteString(dimStyle.Render("  (no dependencies)") + "\n")
	}
	for i, d := range cfg.Dependencies {
		status := active.DependencyStatus(m.pm, d)
		line := fmt.Sprintf("%-18s %-10s %s", d.Name, d.Version, status)
		if i == m.depsCursor {
			sb.WriteString(cursorStyle.Render(" ▶ "+line) + "\n")
		} else {
			style := normalStyle
			if status != "installed" {
				style = errorStyle
			}
			sb.WriteString(style.Render("   "+line) + "\n")
		}
	}

	if n := len(cfg.LegacyDeps.IncDirs) + len(cfg.LegacyDeps.LibDirs); n > 0 {
		sb.WriteString(dimStyle.Render(fmt.Sprintf("\n  %d extra dir(s) registered by packages", n)))
	}

	sb.WriteString(helpStyle.Render("\n↑↓ select • a add • r reinstall • u reinstall all • d remove • esc back"))
	sb.WriteString(m.renderStatus())
	return sb.String()
}

// renderPkgList 安装页：每个包一行（默认显示最新版本），回车装进当前项目。
func (m MainModel) renderPkgList() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Install Package") + "\n\n")
	sb.WriteString(dimStyle.Render("Repository: "+m.pm.RepoDir()) + "\n\n")

	if len(m.repoSummaries) == 0 {
		sb.WriteString("No packages in repository.\n")
		sb.WriteString(dimStyle.Render("\nCreate one with: pm pkg create <dir>"))
		sb.WriteString(m.renderStatus())
		return sb.String()
	}

	installed := map[string]string{}
	if active := m.solution.ActiveProject(); active != nil {
		for _, d := range active.Config().Dependencies {
			installed[d.Name] = d.Version
		}
	}

	for i, s := range m.repoSummaries {
		desc := s.Latest.Manifest.Description
		if desc == "" {
			desc = "-"
		}

		tags := make([]string, 0, 3)
		if len(s.Versions) > 1 {
			tags = append(tags, fmt.Sprintf("%d versions", len(s.Versions)))
		}
		if m.pm.IsBuiltin(s.Latest.File) {
			tags = append(tags, "builtin")
		}
		if cur, ok := installed[s.Name]; ok {
			if cur == s.Latest.Manifest.Version {
				tags = append(tags, "installed")
			} else {
				tags = append(tags, fmt.Sprintf("upgrade %s -> %s", cur, s.Latest.Manifest.Version))
			}
		}
		tag := ""
		if len(tags) > 0 {
			tag = "  [" + strings.Join(tags, ", ") + "]"
		}

		line := fmt.Sprintf("%-18s %-10s %s%s", s.Name, s.Latest.Manifest.Version, desc, tag)
		if i == m.pkgCursor {
			sb.WriteString(cursorStyle.Render(" ▶ "+line) + "\n")
		} else {
			sb.WriteString(normalStyle.Render("   "+line) + "\n")
		}
	}

	sb.WriteString(helpStyle.Render("\n↑↓ select • enter install latest • v versions • esc back"))
	sb.WriteString(m.renderStatus())
	return sb.String()
}

// renderPkgVersions 版本页：列出选中包的所有版本，回车安装指定版本。
func (m MainModel) renderPkgVersions() string {
	var sb strings.Builder
	title := "Package Versions"
	if m.pkgVersionOwner != "" {
		title += ": " + m.pkgVersionOwner
	}
	sb.WriteString(titleStyle.Render(title) + "\n\n")
	sb.WriteString(dimStyle.Render("Repository: "+m.pm.RepoDir()) + "\n\n")

	if len(m.repoPkgs) == 0 {
		sb.WriteString("No versions available.\n")
		sb.WriteString(helpStyle.Render("\nesc back"))
		sb.WriteString(m.renderStatus())
		return sb.String()
	}

	installed := map[string]string{}
	if active := m.solution.ActiveProject(); active != nil {
		for _, d := range active.Config().Dependencies {
			installed[d.Name] = d.Version
		}
	}

	for i, p := range m.repoPkgs {
		desc := p.Manifest.Description
		if desc == "" {
			desc = "-"
		}

		tags := make([]string, 0, 3)
		if i == 0 {
			tags = append(tags, "latest")
		}
		if m.pm.IsBuiltin(p.File) {
			tags = append(tags, "builtin")
		}
		if cur, ok := installed[p.Manifest.Name]; ok && cur == p.Manifest.Version {
			tags = append(tags, "installed")
		}
		tag := ""
		if len(tags) > 0 {
			tag = "  [" + strings.Join(tags, ", ") + "]"
		}

		line := fmt.Sprintf("%-10s %-10s %s%s", p.Manifest.Version, core.FormatSize(p.Size), desc, tag)
		if i == m.pkgCursor {
			sb.WriteString(cursorStyle.Render(" ▶ "+line) + "\n")
		} else {
			sb.WriteString(normalStyle.Render("   "+line) + "\n")
		}
	}

	sb.WriteString(helpStyle.Render("\n↑↓ select • enter install • esc back"))
	sb.WriteString(m.renderStatus())
	return sb.String()
}

func (m MainModel) renderCfgEditPage() string {
	var sb strings.Builder
	title := "Edit Libraries"
	switch m.cfgEditMode {
	case "lib_dirs":
		title = "Edit Library Dirs"
	case "inc_dirs":
		title = "Edit Include Dirs"
	case "src_dirs":
		title = "Edit Source Dirs"
	}
	sb.WriteString(titleStyle.Render(title) + "\n\n")

	items := m.getEditList()
	for i, item := range items {
		if i == m.cfgEditCursor && m.cfgEditFocus == 0 {
			sb.WriteString(cursorStyle.Render(fmt.Sprintf(" ▶ %s", item)) + "\n")
		} else {
			sb.WriteString(normalStyle.Render(fmt.Sprintf("   %s", item)) + "\n")
		}
	}
	if m.cfgEditCursor == len(items) && m.cfgEditFocus == 0 {
		sb.WriteString(cursorStyle.Render(" ▶ [+ Add New]") + "\n")
	} else {
		sb.WriteString(normalStyle.Render("   [+ Add New]") + "\n")
	}

	if m.cfgEditFocus == 1 {
		isEdit := m.cfgEditCursor < len(items)
		prompt := "Input (new item): "
		if isEdit {
			prompt = "Input (edit): "
		}
		sb.WriteString("\n  " + prompt + string(m.cfgEditRunes) + "_\n")
		sb.WriteString(dimStyle.Render("  enter=save  esc=cancel"))
	} else {
		hint := "↑↓ select • a add • d delete • enter edit • esc back"
		if m.directMode {
			hint = "↑↓ select • a add • d delete • enter edit • q quit"
		}
		sb.WriteString("\n" + helpStyle.Render(hint))
	}
	sb.WriteString(m.renderStatus())
	return sb.String()
}

func RunTUI(s *core.Solution) error {
	m := NewMainModel(s)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// openInEditor 用编辑器（默认 VS Code 的 code 命令）打开项目目录。
// 命令不存在或启动失败时返回错误，由调用方决定如何提示。
// 可通过 PM_EDITOR 环境变量指定其它编辑器命令，如 PM_EDITOR="code-insiders"。
func openInEditor(path string) error {
	exe, extraArgs := editorCommand()
	bin, err := exec.LookPath(exe)
	if err != nil {
		return fmt.Errorf("editor %q not found in PATH (VS Code users: install it or put 'code' in PATH; you can also set PM_EDITOR)", exe)
	}

	args := append(extraArgs, path)
	cmd := exec.Command(bin, args...)
	// 后台启动，不等待：code CLI 会把请求交给 VS Code 实例后自行退出
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch %s: %w", exe, err)
	}
	return nil
}

// editorCommand 返回打开项目的编辑器命令与附加参数（默认 code，可用 PM_EDITOR 覆盖）
func editorCommand() (string, []string) {
	if v := strings.TrimSpace(os.Getenv("PM_EDITOR")); v != "" {
		fields := strings.Fields(v)
		return fields[0], fields[1:]
	}
	return "code", nil
}

// RunProjectList 直接进入项目列表页，q/esc 直接退出；回车选择项目后
// 切换活动项目并调用编辑器（默认 VS Code）打开。
func RunProjectList(s *core.Solution) error {
	m := NewMainModel(s)
	m.directMode = true
	m.page = PageList
	if m.solution.Size() == 0 {
		fmt.Println("No projects loaded. Use 'pm tui' to load projects.")
		return nil
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return err
	}
	if mm, ok := final.(MainModel); ok && mm.openInEditorPath != "" {
		fmt.Printf("Opening %s in editor...\n", mm.openInEditorPath)
		if err := openInEditor(mm.openInEditorPath); err != nil {
			// 活动项目已切换成功，仅提示打开编辑器失败，不影响退出码
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}
	return nil
}

// RunConfig 直接进入配置页面，q/esc 直接退出
func RunConfig(s *core.Solution) error {
	m := NewMainModel(s)
	m.directMode = true
	active := m.solution.ActiveProject()
	if active == nil {
		fmt.Println("No active project. Use 'pm tui' to select a project first.")
		return nil
	}
	m.page = PageConfig
	m.editingCfg = active.Config()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// RunBuild 增量构建（不清理）
func RunBuild(s *core.Solution) error {
	active := s.ActiveProject()
	if active == nil {
		return fmt.Errorf("no active project")
	}
	return active.Build(false)
}

// RunClean 清理当前活动项目的编译产物
func RunClean(s *core.Solution) error {
	active := s.ActiveProject()
	if active == nil {
		return fmt.Errorf("no active project")
	}
	return active.Clean()
}

// RunCleanRebuild 清理并重建
func RunCleanRebuild(s *core.Solution) error {
	active := s.ActiveProject()
	if active == nil {
		return fmt.Errorf("no active project")
	}
	if err := active.Clean(); err != nil {
		return err
	}
	return active.Build(false)
}
