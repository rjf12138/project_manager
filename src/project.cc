#include "project.h"
#include "exe_shell.h"
#include "json/wejson.h"

using namespace my_protocol;

Project::Project(string name, string path)
:name_(name), path_(path)
{

}

Project::~Project(void)
{

}

int 
Project::create_project_dir(void)
{
    string result;

    exe_shell_cmd(result, "mkdir %s/%s/", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/config", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/doc", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/inc", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/lib", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/lib/debug", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/lib/release", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/main", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/src", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/output", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/output/debug", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/output/debug/bin", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/output/debug/lib", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/output/release", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/output/release/bin", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/output/release/lib", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/.proj_config", path_.c_str(), name_.c_str());
    exe_shell_cmd(result, "mkdir %s/%s/.vscode", path_.c_str(), name_.c_str());

    return 0;
}

int Project::generate_config(void)
{
    WeJson config;

    config["project_name"] = name_;
    config["project_path"] = path_ + name_;

    string result;
    exe_shell_cmd(result, "echo \"%s\" > project_config.json", config.generate());

    return 0;
}