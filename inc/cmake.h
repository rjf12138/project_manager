#ifndef __CMAKE_H__
#define __CMAKE_H__

#include "basic_head.h"
#include "system_utils.h"
#include "json/wejson.h"

class CMake {
public:
    CMake(void);
    ~CMake(void);

    int load_project(string project_path);
    

private:
    string cmake_bin_path_;
    string project_path_;
    vector<string> _headers;
    vector<string> _sources;
    vector<string> _libraries;

    WeJson proj_config_;
};

#endif