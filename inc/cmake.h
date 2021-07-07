#ifndef __CMAKE_H__
#define __CMAKE_H__

#include "project.h"

class CMake {
public:
    CMake(Project &proj);
    ~CMake(void);
    
    int build_project(bool rebuild);
    int clean_project(void);
    int install_project(void);
    int create_top_level_cmakefile(void);
    int create_sub_cmakefile(string sub_module_path, string module_name); 
    // 创建库时可以遍历所有的源文件目录获取源文件，然后都添加进去
private:
    string cmake_bin_path_;
    string project_path_;
    string project_install_path_;
    string name_;

    WeJson *proj_config_;
    string old_workspace_path_;
};

#endif