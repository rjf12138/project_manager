#ifndef __CMAKE_H__
#define __CMAKE_H__

#include "project.h"

class CMake {
public:
    CMake(Project &proj);
    ~CMake(void);
    

private:
    string cmake_bin_path_;
    string project_path_;
    WeJson *proj_config_;
};

#endif