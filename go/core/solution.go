package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ConfigRoot 返回全局配置根目录 ~/.project_manager
func ConfigRoot() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	return filepath.Join(home, ".project_manager")
}

// Solution 管理多个 Project 的生命周期，持久化打开状态
type Solution struct {
	configDir         string
	projects          map[string]*Project // key = 项目路径
	activeProjectPath string
}

// NewSolution 创建新的 Solution 实例
func NewSolution() *Solution {
	return &Solution{
		projects: make(map[string]*Project),
	}
}

// Init 初始化（创建或加载 ~/.project_manager/ 下的持久化数据）
func (s *Solution) Init() error {
	s.configDir = ConfigRoot()

	if err := os.MkdirAll(s.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return s.loadState()
}

// ConfigDir 返回配置目录
func (s *Solution) ConfigDir() string {
	return s.configDir
}

// CreateProject 创建新项目
func (s *Solution) CreateProject(projName, parentPath string) error {
	fullPath, err := CreateProject(projName, parentPath)
	if err != nil {
		return err
	}

	proj, err := LoadProject(fullPath)
	if err != nil {
		return err
	}

	s.projects[fullPath] = proj
	s.activeProjectPath = fullPath

	return s.saveState()
}

// LoadProject 加载已有项目
func (s *Solution) LoadProject(projPath string) error {
	if _, ok := s.projects[projPath]; ok {
		s.activeProjectPath = projPath
		return s.saveState()
	}

	proj, err := LoadProject(projPath)
	if err != nil {
		return err
	}

	s.projects[projPath] = proj
	s.activeProjectPath = projPath

	return s.saveState()
}

// CloseProject 从解决方案中移除项目
func (s *Solution) CloseProject(projPath string) error {
	delete(s.projects, projPath)

	if s.activeProjectPath == projPath {
		s.activeProjectPath = ""
		for p := range s.projects {
			s.activeProjectPath = p
			break
		}
	}

	return s.saveState()
}

// ActiveProject 获取当前活动项目
func (s *Solution) ActiveProject() *Project {
	if proj, ok := s.projects[s.activeProjectPath]; ok {
		return proj
	}
	return nil
}

// SetActiveProject 设置活动项目
func (s *Solution) SetActiveProject(projPath string) {
	if _, ok := s.projects[projPath]; ok {
		s.activeProjectPath = projPath
		s.saveState()
	}
}

// ProjectPaths 获取所有项目路径列表（按路径排序，保证顺序稳定）
func (s *Solution) ProjectPaths() []string {
	result := make([]string, 0, len(s.projects))
	for p := range s.projects {
		result = append(result, p)
	}
	sort.Strings(result)
	return result
}

// GetProject 根据路径获取项目
func (s *Solution) GetProject(projPath string) *Project {
	return s.projects[projPath]
}

// ActiveProjectPath 返回当前活动项目路径
func (s *Solution) ActiveProjectPath() string {
	return s.activeProjectPath
}

// FindProjectPathByName 按项目名或路径查找已加载项目的路径，找不到返回空串
func (s *Solution) FindProjectPathByName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	// 以路径方式匹配
	if _, ok := s.projects[name]; ok {
		return name
	}
	if abs, err := filepath.Abs(name); err == nil {
		if _, ok := s.projects[abs]; ok {
			return abs
		}
	}

	// 以项目名 / 目录名匹配
	for p, proj := range s.projects {
		if proj.Config().Name == name || filepath.Base(p) == name {
			return p
		}
	}

	return ""
}

// FindProjectByName 按项目名或路径查找项目
func (s *Solution) FindProjectByName(name string) *Project {
	p := s.FindProjectPathByName(name)
	if p == "" {
		return nil
	}
	return s.projects[p]
}

// ProjectNameFromPath 从路径中提取项目名称
func (s *Solution) ProjectNameFromPath(projPath string) string {
	if proj, ok := s.projects[projPath]; ok {
		return proj.Config().Name
	}

	// 尝试从配置文件读取（当前目录格式、早期单文件、旧版位置均兼容）
	if data, _, err := readConfigData(projPath); err == nil {
		var cfg ProjectConfig
		if json.Unmarshal(data, &cfg) == nil && cfg.Name != "" {
			return cfg.Name
		}
	}

	// 回退到路径名
	return filepath.Base(projPath)
}

// Size 返回项目数量
func (s *Solution) Size() int {
	return len(s.projects)
}

// ===================== 持久化 =====================

func (s *Solution) saveState() error {
	state := map[string]interface{}{
		"projects":       s.ProjectPaths(),
		"active_project": s.activeProjectPath,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	statePath := filepath.Join(s.configDir, "solution.json")
	return os.WriteFile(statePath, data, 0644)
}

func (s *Solution) loadState() error {
	statePath := filepath.Join(s.configDir, "solution.json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil // 首次启动无状态文件
	}

	var state struct {
		Projects      []string `json:"projects"`
		ActiveProject string   `json:"active_project"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse solution state: %w", err)
	}

	// 加载项目列表
	for _, path := range state.Projects {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		proj, err := LoadProject(path)
		if err != nil {
			continue
		}

		s.projects[path] = proj
	}

	// 恢复活动项目
	if state.ActiveProject != "" {
		if _, ok := s.projects[state.ActiveProject]; ok {
			s.activeProjectPath = state.ActiveProject
		}
	}

	// 没有活动项目时选第一个
	if s.activeProjectPath == "" && len(s.projects) > 0 {
		for p := range s.projects {
			s.activeProjectPath = p
			break
		}
	}

	return nil
}
