package core

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCreateProjectWritesDefaultMain 新建项目必须自带可编译的入口文件，
// 否则默认配置里的 target 指向 main/main.cc 会导致首次构建失败。
func TestCreateProjectWritesDefaultMain(t *testing.T) {
	tmp := t.TempDir()

	full, err := CreateProject("WithMain", tmp)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(full, "main", "main.cc"))
	if err != nil {
		t.Fatalf("default main.cc missing: %v", err)
	}
	if !strings.Contains(string(data), "int main") {
		t.Errorf("default main.cc does not contain a main function:\n%s", data)
	}
}

// TestBuildCaptureEndToEnd 端到端构建：验证输出被捕获、可执行文件生成，
// 且构建过程不会改变当前进程的工作目录。
func TestBuildCaptureEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("cmake"); err != nil {
		t.Skip("cmake not installed")
	}

	tmp := t.TempDir()
	full, err := CreateProject("Buildable", tmp)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	proj, err := LoadProject(full)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}

	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	log, err := proj.BuildCapture(false)
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, log)
	}
	if strings.TrimSpace(log) == "" {
		t.Errorf("expected captured build output, got empty log")
	}
	if exe := proj.ResolveExecutable(); exe == "" {
		t.Errorf("no executable produced\n%s", log)
	}

	after, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Errorf("build changed process cwd: %q -> %q", before, after)
	}

	// RunCommand 交给调用方接管标准输入输出（TUI 用 tea.ExecProcess）
	runCmd, err := proj.RunCommand()
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if runCmd.Dir != full {
		t.Errorf("RunCommand dir = %q, want project root %q", runCmd.Dir, full)
	}
	if runCmd.Stdout != nil || runCmd.Stderr != nil || runCmd.Stdin != nil {
		t.Errorf("RunCommand should leave stdio unset for the caller")
	}

	var runOut bytes.Buffer
	runCmd.Stdout = &runOut
	runCmd.Stderr = &runOut
	if err := runCmd.Run(); err != nil {
		t.Fatalf("run built executable: %v\n%s", err, runOut.String())
	}
	if !strings.Contains(runOut.String(), "Hello from Buildable!") {
		t.Errorf("unexpected program output: %q", runOut.String())
	}
}
