#include "project.h"
#include "exe_shell.h"
#include "system_utils.h"

Project::Project(string name, string path)
:name_(name), path_(path)
{
    project_path_ = path_ + "/" + name_;
}

Project::~Project(void)
{

}

int 
Project::create_project_dir(void)
{
    string result;

    exe_shell_cmd(result, "mkdir %s/", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/config_", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/doc", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/inc", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/lib", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/lib/debug", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/lib/release", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/main", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/src", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/output", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/output/debug", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/output/debug/bin", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/output/debug/lib", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/output/release", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/output/release/bin", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/output/release/lib", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/.proj_config", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/.vscode", project_path_.c_str());

    string project_config_path = project_path_ + "/project_config.json";
    this->generate_project_config(project_config_path);

    return 0;
}

int Project::generate_project_config(string path)
{
    string result;
    ByteBuffer buffer;
    WeJson arr("[]");

    config_["project_name"] = name_;
    config_["project_path"] = path_ + "/" + name_;

    exe_shell_cmd(result, "uuidgen | tr -d '\n'");
    config_["project_uuid"] = result;

    arr.parse("[]");
    arr.add("./lib/debug/");
    arr.add("./lib/release/");
    arr.add("./output/debug/lib/");
    arr.add("./output/release/lib/");
    config_["library_dir"] = arr;
    arr.clear();

    arr.parse("[]");
    arr.add("./src/");
    arr.add("./main/");
    config_["src_dir"] = arr;
    arr.clear();

    arr.parse("[]");
    arr.add("./inc/");
    config_["inc_dir"] = arr;
    arr.clear();

    arr.parse("[]");
    arr.add("g++");
    config_["choose_compiler"] = arr;
    arr.clear();

    config_["release_compile_option"] = "-O2 -Wall -std=c++11";
    arr.parse("[]");
    arr.add("gmock");
    arr.add("gtest");
    config_["release_link_library"] = arr;
    arr.clear();

    config_["debug_compile_option"] = "-O0 -Wall -g -ggdb -std=c++11";
    arr.parse("[]");
    arr.add("gmock_main");
    arr.add("gtest_main");
    config_["debug_link_library"] = arr;
    arr.clear();

    config_["compile_method"] = "debug";
    config_["program_run_arg"] = "";


    config_["compiler"] = "g++";
    config_["main_cpp_file"] = "";
    config_["generate_file_type"] = "exe";

    buffer.write_string(config_.format_json());
    system_utils::Stream project_config;
    project_config.open(path);
    project_config.write(buffer, buffer.data_size());

    return 0;
}

int Project::generate_vscode_config(string path)
{
    string result;

    exe_shell_cmd(result, "echo \"{\"                                                                > %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"    // See https://go.microsoft.com/fwlink/?LinkId=733558\"        >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"    // for the documentation about the tasks.json format\"         >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"        \"tasks\": [\"                                             >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"            {\"                                                    >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"                \"type\": \"shell\",\"                             >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"                \"label\": \"compile script\",\"                   >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"                \"command\": \"project\",\"                        >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"                \"args\": [\"                                      >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"                    \"-r\"\"                                       >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"                ],\"                                               >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"                \"options\": {\"                                   >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"                    \"cwd\": \"%s\"\"                              >> %s/tasks.json", path.c_str(), project_path_.c_str());
    exe_shell_cmd(result, "echo \"                },\"                                               >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"                \"group\": {\"                                     >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"                    \"kind\": \"build\",\"                         >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"                    \"isDefault\": true\"                          >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"                }\"                                                >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"            }\"                                                    >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"        ],\"                                                       >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"        \"version\": \"2.0.0\"\"                                   >> %s/tasks.json", path.c_str());
    exe_shell_cmd(result, "echo \"    }\"                                                            >> %s/tasks.json", path.c_str());

    JsonString program_arg = config_["program_run_arg"];
    JsonString program_main_file = config_["main_cpp_file"];
    exe_shell_cmd(result, "echo \"{\"                                                                > %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"   // Use IntelliSense to learn about possible attributes.\"       >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"   // Hover to view descriptions of existing attributes.\"         >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"   // For more information, visit: \"                              >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"   // https://go.microsoft.com/fwlink/?linkid=830387\"             >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"    \"version\": \"0.2.0\",\"                                      >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"    \"configurations\": [\"                                        >> %s/launch.json", path.c_str()); 
    exe_shell_cmd(result, "echo \"        {\"                                                        >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"            \"name\": \"gdb_debug\",\"                             >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"            \"type\": \"cppdbg\",\"                                >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"            \"request\": \"launch\",\"                             >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo %s | awk -F[.] '{print $1}'", program_main_file.value());
    exe_shell_cmd(result, "echo \"            \"program\": \"%s/%s\",\"                              >> %s/launch.json", output_bin_path_.c_str(), result.c_str());
    if (program_arg == "") {
        exe_shell_cmd(result, "echo \"            \"args\": [],\"                                    >> %s/launch.json", path.c_str());
    } else {
        exe_shell_cmd(result, "echo \"            \"args\": [\"%s\"],\"                              >> %s/launch.json", path.c_str(), program_arg.value().c_str());
    }
    exe_shell_cmd(result, "echo \"            \"stopAtEntry\": false,\"                              >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"            \"cwd\": \"%s\",\"                                     >> %s/launch.json", path.c_str(), project_path_.c_str());
    exe_shell_cmd(result, "echo \"            \"environment\": [],\"                                 >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"            \"externalConsole\": false,\"                          >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"            \"MIMode\": \"gdb\",\"                                 >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"            \"setupCommands\": [\"                                 >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"                {\"                                                >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"                    \"description\": \"\",\"                       >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"                    \"text\": \"-enable-pretty-printing\",\"       >> %s/launch.json", path.c_str());    
    exe_shell_cmd(result, "echo \"                    \"ignoreFailures\": true\"                     >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"                }\"                                                >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"            ]\"                                                    >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"        }\"                                                        >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"    ]\"                                                            >> %s/launch.json", path.c_str());
    exe_shell_cmd(result, "echo \"}\"                                                                >> %s/launch.json", path.c_str());
}