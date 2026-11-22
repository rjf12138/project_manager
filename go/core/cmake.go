package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

// cmakePath 把配置里的目录转成 CMake 可用的路径：
// 相对路径按项目根解析，绝对路径原样使用。
func cmakePath(dir string) string {
	if filepath.IsAbs(dir) {
		return filepath.ToSlash(dir)
	}
	return "${CMAKE_SOURCE_DIR}/" + filepath.ToSlash(dir)
}

// GenerateCMakeLists 根据项目配置生成 CMakeLists.txt 内容
func GenerateCMakeLists(cfg ProjectConfig, dirInfo ProjectDirInfo) string {
	var sb strings.Builder

	buildMode := string(cfg.BuildMode)

	// 基本信息
	sb.WriteString(fmt.Sprintf("cmake_minimum_required(VERSION %s)\n\n", cfg.CMakeMinimum))
	sb.WriteString(fmt.Sprintf("project(%s VERSION %s LANGUAGES %s)\n\n", cfg.Name, cfg.Version, cfg.Language))

	// C++ 标准
	if cfg.Language == "CXX" && cfg.CXXStandard > 0 {
		sb.WriteString(fmt.Sprintf("set(CMAKE_CXX_STANDARD %d)\n", cfg.CXXStandard))
		sb.WriteString("set(CMAKE_CXX_STANDARD_REQUIRED ON)\n\n")
	}

	// Debug 后缀
	sb.WriteString("set(CMAKE_DEBUG_POSTFIX d)\n\n")

	// 编译器标志（按当前构建模式应用 build_options）
	var flags []string
	if cfg.BuildMode == BuildModeDebug {
		flags = appendUnique(flags, "-g", "-ggdb")
	} else {
		flags = appendUnique(flags, "-O3")
	}
	flags = appendUnique(flags,
		"-DCHECK_PTHREAD_RETURN_VALUE",
		"-D_FILE_OFFSET_BITS=64",
		"-march=native",
		"-rdynamic",
	)

	modeOpts := cfg.BuildOptions.Debug
	if cfg.BuildMode == BuildModeRelease {
		modeOpts = cfg.BuildOptions.Release
	}
	flags = appendUnique(flags, modeOpts.Optimization)
	for _, d := range modeOpts.Defines {
		flags = appendUnique(flags, "-D"+d)
	}

	sb.WriteString("# Compiler flags\n")
	sb.WriteString("set(CXX_FLAGS\n")
	for _, f := range flags {
		sb.WriteString("    " + f + "\n")
	}
	sb.WriteString(")\n")
	sb.WriteString("string(REPLACE \";\" \" \" CMAKE_CXX_FLAGS \"${CXX_FLAGS}\")\n\n")

	// 输出目录
	sb.WriteString("# Output directories\n")
	sb.WriteString(fmt.Sprintf(
		"set(EXECUTABLE_OUTPUT_PATH ${CMAKE_SOURCE_DIR}/%s/%s/bin)\n",
		cfg.OutputDir, buildMode))
	sb.WriteString(fmt.Sprintf(
		"set(LIBRARY_OUTPUT_PATH ${CMAKE_SOURCE_DIR}/%s/%s/lib)\n\n",
		cfg.OutputDir, buildMode))

	// 收集所有源文件
	sb.WriteString("# Source files\n")
	sb.WriteString("file(GLOB_RECURSE SRC_FILES\n")
	for _, sd := range cfg.SourceDirs {
		sb.WriteString(fmt.Sprintf("    ${CMAKE_SOURCE_DIR}/%s/*.cc\n", sd))
		sb.WriteString(fmt.Sprintf("    ${CMAKE_SOURCE_DIR}/%s/*.cpp\n", sd))
		sb.WriteString(fmt.Sprintf("    ${CMAKE_SOURCE_DIR}/%s/*.cxx\n", sd))
		sb.WriteString(fmt.Sprintf("    ${CMAKE_SOURCE_DIR}/%s/*.c\n", sd))
	}
	sb.WriteString(")\n\n")

	// 头文件目录
	sb.WriteString("# Include directories\n")
	// inc/ 和 extern_inc/ 是默认添加的
	for _, inc := range cfg.IncludeDirs {
		sb.WriteString(fmt.Sprintf("include_directories(${CMAKE_SOURCE_DIR}/%s)\n", inc))
	}
	sb.WriteString(fmt.Sprintf("include_directories(${CMAKE_SOURCE_DIR}/%s)\n", cfg.ExternIncDir))
	// 源文件目录也添加为 include 目录（内部头文件和源文件放在一起）
	for _, sd := range cfg.SourceDirs {
		sb.WriteString(fmt.Sprintf("include_directories(${CMAKE_SOURCE_DIR}/%s)\n", sd))
	}
	// 依赖（安装的包）提供的额外头文件目录
	for _, inc := range cfg.LegacyDeps.IncDirs {
		sb.WriteString(fmt.Sprintf("include_directories(%s)\n", cmakePath(inc)))
	}
	// 额外的头文件目录
	for _, extra := range cfg.ExtraIncludeDirs {
		sb.WriteString(fmt.Sprintf("include_directories(%s)\n", extra))
	}
	sb.WriteString("\n")

	// 库目录
	sb.WriteString("# Link directories\n")
	sb.WriteString(fmt.Sprintf(
		"link_directories(${CMAKE_SOURCE_DIR}/%s/%s)\n", cfg.LibDir, buildMode))
	// 依赖（安装的包）提供的额外库目录
	for _, dir := range cfg.LegacyDeps.LibDirs {
		sb.WriteString(fmt.Sprintf("link_directories(%s)\n", cmakePath(dir)))
	}
	for _, extra := range cfg.ExtraLibDirs {
		sb.WriteString(fmt.Sprintf("link_directories(%s)\n", extra))
	}
	sb.WriteString("\n")

	// 构建目标
	sb.WriteString("# Build targets\n")
	hasTargets := false
	for _, target := range cfg.Targets {
		hasTargets = true
		switch target.Type {
		case "executable":
			sb.WriteString(fmt.Sprintf("add_executable(%s", target.Name))
			if len(target.SourceFiles) > 0 {
				for _, sf := range target.SourceFiles {
					sb.WriteString(fmt.Sprintf("\n    ${CMAKE_SOURCE_DIR}/%s", sf))
				}
			} else if cfg.MainFile != "" {
				sb.WriteString(fmt.Sprintf("\n    ${CMAKE_SOURCE_DIR}/main/%s", cfg.MainFile))
			}
			sb.WriteString("\n    ${SRC_FILES}\n")
			sb.WriteString(")\n\n")

		case "static_lib", "library":
			sb.WriteString(fmt.Sprintf("add_library(%s STATIC\n    ${SRC_FILES}\n)\n\n", target.Name))

		case "share_lib":
			sb.WriteString(fmt.Sprintf("add_library(%s SHARED\n    ${SRC_FILES}\n)\n\n", target.Name))
		}
	}

	// 如果没有定义 targets，创建默认可执行目标
	if !hasTargets {
		sb.WriteString(fmt.Sprintf("add_executable(%s\n", cfg.Name))
		if cfg.MainFile != "" {
			sb.WriteString(fmt.Sprintf("    ${CMAKE_SOURCE_DIR}/main/%s\n", cfg.MainFile))
		}
		sb.WriteString("    ${SRC_FILES}\n")
		sb.WriteString(")\n\n")
	}

	// 为每个 target 添加头文件目录和链接库
	for _, target := range cfg.Targets {
		sb.WriteString(fmt.Sprintf("# Include dirs for target %s\n", target.Name))
		for _, inc := range cfg.IncludeDirs {
			sb.WriteString(fmt.Sprintf("target_include_directories(%s PRIVATE ${CMAKE_SOURCE_DIR}/%s)\n",
				target.Name, inc))
		}
		sb.WriteString(fmt.Sprintf("target_include_directories(%s PRIVATE ${CMAKE_SOURCE_DIR}/%s)\n",
			target.Name, cfg.ExternIncDir))
		// 源文件目录也添加为 include 目录
		for _, sd := range cfg.SourceDirs {
			sb.WriteString(fmt.Sprintf("target_include_directories(%s PRIVATE ${CMAKE_SOURCE_DIR}/%s)\n",
				target.Name, sd))
		}
		// 依赖（安装的包）提供的额外头文件目录
		for _, inc := range cfg.LegacyDeps.IncDirs {
			sb.WriteString(fmt.Sprintf("target_include_directories(%s PRIVATE %s)\n",
				target.Name, cmakePath(inc)))
		}
		for _, extra := range cfg.ExtraIncludeDirs {
			sb.WriteString(fmt.Sprintf("target_include_directories(%s PRIVATE %s)\n",
				target.Name, extra))
		}

		if len(target.LinkLibs) > 0 {
			sb.WriteString(fmt.Sprintf("target_link_libraries(%s", target.Name))
			for _, lib := range target.LinkLibs {
				sb.WriteString(fmt.Sprintf("\n    %s", lib))
			}
			sb.WriteString("\n)\n")
		}
		sb.WriteString("\n")
	}

	// 旧版兼容：legacy deps（依赖安装时写入的链接库）
	if len(cfg.LegacyDeps.Libs) > 0 {
		targetNames := make([]string, 0, len(cfg.Targets))
		for _, target := range cfg.Targets {
			targetNames = append(targetNames, target.Name)
		}
		// 未定义 targets 时使用默认可执行目标名
		if len(targetNames) == 0 {
			targetNames = append(targetNames, cfg.Name)
		}

		sb.WriteString("# Legacy dependencies\n")
		for _, name := range targetNames {
			sb.WriteString(fmt.Sprintf("target_link_libraries(%s", name))
			for _, lib := range cfg.LegacyDeps.Libs {
				sb.WriteString(fmt.Sprintf("\n    %s", lib))
			}
			sb.WriteString("\n)\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
