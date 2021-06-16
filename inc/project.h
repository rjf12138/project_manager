#ifndef __PROJECT_H__
#define __PROJECT_H__

#include "basic_head.h"
#include "project_window.h"
#include "json/wejson.h"

using namespace my_protocol;

class Project {
public:
    Project(void);
    virtual ~Project(void);

    int init(void);

    int load_project(string project_path = "");
    int create_project(void);
    int check_project_opened(void);

    int modify_config(void);

    int generate_project_config(string path);
    int generate_vscode_config(string path);
    
    int pull_file(void);
    int push_file(void);

private:
    bool is_project_opened_;

    string name_;
    string project_install_path_;
    string project_path_;
    string output_bin_path_;
    string output_lib_path_;

    WeJson config_;
    ProjectWindow _window;
};

#endif