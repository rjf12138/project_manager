#include "project.h"
#include "exe_shell.h"
#include "system_utils.h"
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
    string result;
    ByteBuffer buffer;
    WeJson config("{}"), arr("[]");

    config["project_name"] = name_;
    config["project_path"] = path_ + "/" + name_;

    exe_shell_cmd(result, "uuidgen | awk -F[\\n] '{print $1}'");
    config["project_uuid"] = result;

    arr.parse("[]");
    arr.add("./lib/debug/");
    arr.add("./lib/release/");
    arr.add("./output/debug/lib/");
    arr.add("./output/release/lib/");
    config["library_dir"] = arr;
    arr.clear();

    arr.parse("[]");
    arr.add("./src/");
    arr.add("./main/");
    config["src_dir"] = arr;
    arr.clear();

    arr.parse("[]");
    arr.add("./inc/");
    config["inc_dir"] = arr;
    arr.clear();

    arr.parse("[]");
    arr.add("g++");
    config["choose_compiler"] = arr;
    arr.clear();

    config["release_compile_option"] = "-O2 -Wall -std=c++11";
    arr.parse("[]");
    arr.add("gmock");
    arr.add("gtest");
    config["release_link_library"] = arr;
    arr.clear();

    config["debug_compile_option"] = "-O0 -Wall -g -ggdb -std=c++11";
    arr.parse("[]");
    arr.add("gmock_main");
    arr.add("gtest_main");
    config["debug_link_library"] = arr;
    arr.clear();

    config["compile_method"] = "debug";
    config["program_run_arg"] = "";


    config["compiler"] = "g++";
    config["main_cpp_file"] = "";
    config["generate_file_type"] = "exe";

    buffer.write_string(config.format_json());
    system_utils::Stream project_config;
    project_config.open("./project_config.json");
    project_config.write(buffer, buffer.data_size());

    return 0;
}