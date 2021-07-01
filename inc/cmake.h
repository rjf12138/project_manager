#ifndef __CMAKE_H__
#define __CMAKE_H__

#include "project.h"

class CMake {
public:
    CMake(Project &proj);
    ~CMake(void);
    
    int create_top_level_cmakefile(void);
    int create_sub_cmakefile(void); 
    // 创建库时可以遍历所有的源文件目录获取源文件，然后都添加进去
private:
    string cmake_bin_path_;
    string project_path_;
    string name_;

    WeJson *proj_config_;
    string old_workspace_path_;
};

#endif