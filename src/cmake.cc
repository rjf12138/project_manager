#include "cmake.h"
#include "system_utils.h"
#include <dirent.h>

CMake::CMake(Project &proj)
{
    name_ = proj.get_project_name();
    proj_config_ = proj.get_project_config();
    cmake_bin_path_ = proj.get_project_install_path() + "/program/bin/cmake/cmake";
    JsonString path = (*proj_config_)["Path"];

    char buffer[1024] = "";
    getcwd(buffer, 1024);
    old_workspace_path_ = buffer;
    chdir(path.value().c_str());
}

CMake::~CMake(void)
{
    chdir(old_workspace_path_.c_str());
}


int CMake::create_top_level_cmakefile(void)
{
    JsonString compile_method = (*proj_config_)["CompilationMethod"];
    JsonArray inc_dirs = (*proj_config_)["HeaderFileDirectoryListing"];
    JsonArray src_dirs = (*proj_config_)["SourceFileDirectoryListing"];
    JsonArray lib_dirs = (*proj_config_)["LibraryDirectoryListing"];
    JsonArray libs = (*proj_config_)["LibraryListing"][compile_method.value()];

    ostringstream cmakefile_stream;
    cmakefile_stream << "cmake_minimum_required(VERSION 3.10)" << std::endl << std::endl;
    cmakefile_stream << "# 设置项目名称" << std::endl;
    cmakefile_stream << "project(" << name_ << ")" << std::endl << std::endl;

    cmakefile_stream << "# 指定c++标准: 默认是 c++11" << std::endl;
    cmakefile_stream << "set(CMAKE_CXX_STANDARD 11)" << std::endl;
    cmakefile_stream << "set(CMAKE_CXX_STANDARD_REQUIRED True)" << std::endl << std::endl;

    cmakefile_stream << "set(CMAKE_DEBUG_POSTFIX d)" << std::endl << std::endl;

    cmakefile_stream << "# 设置 c++ 警告标志" << std::endl;
    cmakefile_stream << "set(gcc_like_cxx \"$<COMPILE_LANG_AND_ID:CXX,ARMClang,AppleClang,Clang,GNU>\")" << std::endl;
    cmakefile_stream << "set(msvc_cxx \"$<COMPILE_LANG_AND_ID:CXX,MSVC>\")" << std::endl << std::endl;
    cmakefile_stream << "target_compile_options(tutorial_compiler_flags INTERFACE" << std::endl;
    cmakefile_stream << "\t\t$<${gcc_like_cxx}:$<BUILD_INTERFACE:-Wall;-Wextra;-Wshadow;-Wformat=2;-Wunused>>" << std::endl;
    cmakefile_stream << "\t\t$<${msvc_cxx}:$<BUILD_INTERFACE:-W3>>)" << std::endl << std::endl;

    // 添加头文件目录
    for (int i = 0; i < inc_dirs.size(); ++i) {
        JsonString target_path = inc_dirs[i];
        cmakefile_stream << "target_include_directories(" << name_ << " PUBLIC \"" << target_path.value() << "}\")" << std::endl;
    }
    cmakefile_stream << std::endl;

    // 添加库目录
    for (int i = 0; i < lib_dirs.size(); ++i) {
        JsonString target_path = lib_dirs[i];
        cmakefile_stream << "target_link_libraries(" << name_ << " PUBLIC \"" << target_path.value() << "}\")" << std::endl;
    }
    cmakefile_stream << std::endl;

    // 添加源文件目录
    for (int i = 0; i < src_dirs.size(); ++i) {
        JsonString target_path = src_dirs[i];
        cmakefile_stream << "add_subdirectory(" << name_ << ")" << std::endl;
    }
    cmakefile_stream << std::endl;

    ByteBuffer buffer;
    buffer.write_string(cmakefile_stream.str());
    system_utils::Stream top_cmakefile;
    top_cmakefile.open("./CMakeLists.txt");
    top_cmakefile.write(buffer, buffer.data_size());

    return 0;
}