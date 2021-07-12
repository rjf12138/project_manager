#ifndef __CMAKE_H__
#define __CMAKE_H__

#include "project.h"

class CMake {
public:
    CMake(Project &proj);
    ~CMake(void);
    
    // 生成项目
    int build_project(bool rebuild);
    // 清空项目
    int clean_project(void);
    // 创建安装的环境添加配置文件
    int create_install_env(void);
    // 创建顶层cmake文件
    int create_top_level_cmakefile(void);
    // 创建下级目录cmake文件
    int create_sub_cmakefile(string sub_module_path, string module_name); 
    
private:
    string cmake_bin_path_;
    string project_path_;
    string project_install_path_;
    string name_;

    WeJson *proj_config_;
    string old_workspace_path_;
};

#endif