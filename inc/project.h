#ifndef __PROJECT_H__
#define __PROJECT_H__

#include "basic_head.h"
#include "json/wejson.h"

using namespace my_protocol;

class Project {
public:
    Project(string name, string path);
    ~Project(void);

    int load_project(string project_path);
    int create_project_dir(void);

    int generate_project_config(string path);
    int generate_vscode_config(string path);

    int print_proj_name(void);
private:
    string name_;
    string project_located_path_;
    string project_path_;
    string output_bin_path_;
    string output_lib_path_;
    WeJson config_;
};

#endif