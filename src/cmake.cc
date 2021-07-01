#include "cmake.h"

CMake::CMake(Project &proj)
{
    proj_config_ = proj.get_project_config();
    cmake_bin_path_ = proj.get_project_install_path() + "/program/bin/cmake/cmake";
}

CMake::~CMake(void)
{
}

