#include "project.h"
#include "exe_shell.h"
#include "system_utils.h"

Project::Project(void)
    :is_project_opened_(false), 
     name_(""), 
     project_located_path_("")
{
}

Project::~Project(void)
{

}

int 
Project::load_project(string project_path)
{
    LOG_GLOBAL_TRACE("Load_project: %s", project_path.c_str());
    bool is_new_project = false;
    ByteBuffer buffer;
    system_utils::Stream project_paths;
    int ret = project_paths.open("./config/project_path.json");
    if (ret == -1) {
        return -1;
    }
    project_paths.read(buffer, project_paths.file_size());
    WeJson js_project_paths(buffer);
    
    if (project_path == "") { // 参数为空则从缓存文件中读取
        vector<string> names={"Input Project Path",}, paths = {""};
        JsonString js_name = js_project_paths["recent_open_project"]["name"];
        JsonString js_path = js_project_paths["recent_open_project"]["path"];
        names.push_back(js_name.value());
        paths.push_back(js_path.value());
        
        auto iter = js_project_paths["project_paths"].begin();
        auto iter_end = js_project_paths["project_paths"].end();
        for (; iter != iter_end; ++iter) {
            js_name = iter.second()["name"];
            js_path = iter.second()["path"];

            if (js_path.value() == paths[1]) { // 最近打开过的项目，已经添加进去了，不用在加了
                continue;
            }

            names.push_back(js_name.value());
            paths.push_back(js_path.value());
        }

        project_path_ = _window.display_menu(names, paths);
        if (project_path_ == ProjWin_InputPath) { // 新添加的项目路径
            _window.get_input(project_path_, "Input new project path");
            is_new_project = true;
        }

        ByteBuffer config_buf;
        system_utils::Stream project_config;
        string config_path = project_path_ + "/.proj_config/project_config.json";
        ret = project_config.open(config_path);
        if (ret == -1) {
            LOG_GLOBAL_ERROR("Load project failed： Can not load project config: %s.", config_path.c_str());
            return -1;
        }
        project_config.read(config_buf, project_config.file_size());
        config_.parse(config_buf);

        JsonString name = config_["project_name"];
        name_ = name.value();
    } else {
        project_path_ = project_path;
        is_new_project = true;
    }
    
    if (is_new_project == true) {
        WeJson new_project("{}");
        new_project["name"] = name_;
        new_project["path"] = project_path_;
        js_project_paths["project_paths"].add(new_project);
    }

    js_project_paths["recent_open_project"]["name"] = name_;
    js_project_paths["recent_open_project"]["path"] = project_path_;

    buffer.clear();
    buffer.write_string(js_project_paths.format_json());
    project_paths.clear_file();
    project_paths.write(buffer, buffer.data_size());

    is_project_opened_ = true;
    return 0;
}

int 
Project::create_project(void)
{
    _window.get_input(name_, "New Project Name");
    if (name_ == "") {
        LOG_GLOBAL_ERROR("Project name can't be empty");
        return -1;
    }

    _window.get_input(project_path_, "The new project is located at");
    if (project_path_ == "") {
        LOG_GLOBAL_ERROR("Project path can't be empty");
        return -1;
    }

    project_path_ = project_path_ + "/" + name_;
    string result;
    exe_shell_cmd(result, "mkdir %s/", project_path_.c_str());
    exe_shell_cmd(result, "mkdir %s/config", project_path_.c_str());
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

    string project_config_path = project_path_ + "/.proj_config/project_config.json";
    this->generate_project_config(project_config_path);
    this->load_project(project_path_);

    return 0;
}

int Project::generate_project_config(string path)
{
    string result;
    ByteBuffer buffer;
    WeJson arr("[]");
    config_.parse("{}");

    config_["project_name"] = name_;
    config_["project_path"] = project_located_path_ + "/" + name_;

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
    config_["ListOfAvailableCompilers"] = arr;
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

    config_["CompilationMethod"] = "debug";
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

int 
Project::generate_vscode_config(string path)
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
    exe_shell_cmd(result, "echo \"            \"cwd\": \"%s\",\"                                     >> %s/launch.json", output_bin_path_.c_str());
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

    return 0;
}


int 
Project::modify_config(void)
{
    JsonString js_value;
    vector<string> cfg_params;
    vector<string> cfg_values;

    if (is_project_opened_ == false) {
        LOG_GLOBAL_ERROR("No project has been loaded yet");
        return -1;
    }

    cfg_params.push_back("Name");
    cfg_values.push_back(name_);

    cfg_params.push_back("Path");
    cfg_values.push_back(project_path_);

    cfg_params.push_back("UUID");
    js_value = config_["UUID"];
    cfg_values.push_back(js_value.value());

    cfg_params.push_back("Current compiler"); // 选择使用的编译器
    js_value = config_["CurrentCompiler"];
    cfg_values.push_back(js_value.value());

    // cfg_params.push_back("Add compiler"); // 添加编译器
    // js_value = config_["AddCompiler"];
    // cfg_values.push_back(js_value.value());

    // cfg_params.push_back("List of available compilers");
    // js_value = config_["ListOfAvailableCompilers"];
    // cfg_values.push_back(js_value.value());

    cfg_params.push_back("Compilation method"); // Debug or Release
    js_value = config_["CompilationMethod"];
    cfg_values.push_back(js_value.value());

    cfg_params.push_back("Compilation parameters");
    js_value = config_["CompilationParameters"];
    cfg_values.push_back(js_value.value());

    cfg_params.push_back("Generate file type"); // share, static, exe
    js_value = config_["GenerateFileType"];
    cfg_values.push_back(js_value.value());

    cfg_params.push_back("Main file name"); // 主函数所在的文件名称
    js_value = config_["MainFileName"];
    cfg_values.push_back(js_value.value());

    cfg_params.push_back("Library listing"); // 库列表
    js_value = config_["LibraryListing"];
    cfg_values.push_back(js_value.value());

    cfg_params.push_back("Header file directory listing"); // 头文件目录列表
    js_value = config_["HeaderFileDirectoryListing"];
    cfg_values.push_back(js_value.value());

    cfg_params.push_back("Source file directory listing"); // 源文件目录列表
    js_value = config_["SourceFileDirectoryListing"];
    cfg_values.push_back(js_value.value());

    cfg_params.push_back("Library directory listing"); // 库目录列表
    js_value = config_["LibraryDirectoryListing"];
    cfg_values.push_back(js_value.value());

    cfg_params.push_back("Import and export directory"); // 库和头文件导入导出目录
    js_value = config_["ImportAndExportDirectory"];
    cfg_values.push_back(js_value.value());

    cfg_params.push_back("Save");   // 库和头文件导入导出目录
    js_value = config_["!@#$"];     // 随机定的保存字符串
    cfg_values.push_back(js_value.value());

    while (true) {
        string value = _window.display_menu(cfg_params, cfg_values);
        if (value == "") {
            break;
        }

        if (value == "CurrentCompiler") {
            while (true) {
                vector<string> compiler_nums = {"1"};
                vector<string> compiler_names = {"Add new compiler"};
                for (int i = 0; i < config_["ChooseCompiler"].size(); ++i) {
                    compiler_nums.push_back(to_string(i + 2));

                    JsonString tmp_value = config_["ChooseCompiler"][i]["CompilerName"];
                    compiler_names.push_back(tmp_value.value());
                }

                string compiler_name = _window.display_menu(compiler_nums, compiler_names);
                if (compiler_name == "Add new compiler") {
                    WeJson new_compiler("{}");
                    string new_compiler_name;
                    _window.get_input(new_compiler_name, "New compiler Name");
                    if (new_compiler_name == "") {
                        // 什么都不做回到编译器选择界面
                    } else {
                        new_compiler["CompilerName"] = new_compiler_name;
                        string new_compiler_option;
                        _window.get_input(new_compiler_option, "New compiler debug option");
                        new_compiler["DebugOption"] = new_compiler_option;

                        _window.get_input(new_compiler_option, "New compiler release option");
                        new_compiler["ReleaseOption"] = new_compiler_option;

                        config_["ChooseCompiler"].add(new_compiler);
                    }
                } else {
                    config_["CurrentCompiler"] = compiler_name;
                    JsonString compile_method = config_["CompilationMethod"];
                    
                    for (int i = 0; i < config_["ChooseCompiler"].size(); ++i) {
                        JsonString choose_compiler_name = config_["ChooseCompiler"][i]["CompilerName"];
                        if (choose_compiler_name.value() == compiler_name) {
                            if (compile_method.value() == "Release") {
                                config_["CompilationParameters"] = config_["ChooseCompiler"][i]["ReleaseOption"];
                            } else {
                                config_["CompilationParameters"] = config_["ChooseCompiler"][i]["DebugOption"];
                            }
                        }
                    }
                    break;
                }
            }
        } else if (value == "CompilationMethod") {
            vector<string> nums = {"1", "2"};
            vector<string> compile_method = {"Debug", "Release"};
            string method = _window.display_menu(nums, compile_method);
            if (method == "") {
                continue;
            }
            config_["CompilationMethod"] = method;
            
            JsonString current_compile_name = config_["CurrentCompiler"];
            for (int i = 0; i < config_["ChooseCompiler"].size(); ++i) {
                JsonString choose_compiler_name = config_["ChooseCompiler"][i]["CompilerName"];
                if (choose_compiler_name.value() == current_compile_name.value()) {
                    if (method == "Release") {
                        config_["CompilationParameters"] = config_["ChooseCompiler"][i]["ReleaseOption"];
                    } else {
                        config_["CompilationParameters"] = config_["ChooseCompiler"][i]["DebugOption"];
                    }
                }
            }
        } else if (value == "CompilationParameters") {
            JsonString param = config_["CompilationParameters"];
            string ret_param;
            int ret = _window.get_input(ret_param, "Modify compile parameters", param.value());
            if (ret != -1) {
                config_["CompilationParameters"] = ret_param;
            }
        } else if (value == "GenerateFileType") {
            vector<string> keys = {"1", "2", "3"};
            vector<string> values = {"exe", "static", "share"};

            string type = _window.display_menu(keys, values);
            config_["GenerateFileType"] = type;
        } else if (value == "MainFileName") {
            vector<string> keys, values;
            for (int i = 0; i < config_["MainFileNameList"].size(); ++i) {
                keys.push_back(to_string(i + 1));
                JsonString main_filename = config_["MainFileNameList"][i];
                values.push_back(main_filename.value());
            }
            string main_filename = _window.display_menu(keys, values);
            config_["MainFileName"] = main_filename;
        } else if (value == "LibraryListing") {
            vector<string> keys = {"1", "2", "3", "4"}, values = {"View Libraries", "Add Library", "Remove Library", "Exit"};
            while (true) {
                string operation = _window.display_menu(keys, values);
                if (operation == "View Libraries") {
                    vector<string> nums, library_list;
                    for (int i = 0; i < config_["LibraryListing"].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["LibraryListing"][i];
                        nums.push_back(name.value());
                    }
                    _window.display_menu(nums, library_list);
                } else if (operation == "Add Library") {
                    string library_name;
                    int ret = _window.get_input(library_name, "Input Library name");
                    if (ret == -1 || library_name == "") {
                        continue;
                    }
                    config_["LibraryListing"].add(library_name);
                } else if (operation == "Remove Library") {
                    vector<string> nums, library_list;
                    for (int i = 0; i < config_["LibraryListing"].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["LibraryListing"][i];
                        nums.push_back(name.value());
                    }
                    string library_name = _window.display_menu(nums, library_list);
                    if (library_name == "") {
                        continue;
                    }

                    string title = "Are you sure to delete ";
                    bool is_delete = _window.message(title + library_name);
                    for (int i = 0; i < config_["LibraryListing"].size() && is_delete; ++i) {
                        JsonString name = config_["LibraryListing"][i];
                        if (name.value() == library_name) {
                            config_["LibraryListing"].erase(i);
                            continue;
                        }
                    }
                }
            }
        } else if (value == "HeaderFileDirectoryListing") {
            vector<string> keys = {"1", "2", "3", "4"}, values = {"View header directories", "Add header directory", "Remove header directory", "Exit"};
            while (true) {
                string operation = _window.display_menu(keys, values);
                if (operation == "View header directories") {
                    vector<string> nums, list;
                    for (int i = 0; i < config_["HeaderFileDirectoryListing"].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["HeaderFileDirectoryListing"][i];
                        nums.push_back(name.value());
                    }
                    _window.display_menu(nums, list);
                } else if (operation == "Add header directory") {
                    string name;
                    int ret = _window.get_input(name, "Input Library name");
                    if (ret == -1 || name == "") {
                        continue;
                    }
                    config_["HeaderFileDirectoryListing"].add(name);
                } else if (operation == "Remove header directory") {
                    vector<string> nums, list;
                    for (int i = 0; i < config_["HeaderFileDirectoryListing"].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["HeaderFileDirectoryListing"][i];
                        nums.push_back(name.value());
                    }
                    string del_name = _window.display_menu(nums, list);
                    if (del_name == "") {
                        continue;
                    }

                    string title = "Are you sure to delete ";
                    bool is_delete = _window.message(title + del_name);
                    for (int i = 0; i < config_["HeaderFileDirectoryListing"].size() && is_delete; ++i) {
                        JsonString name = config_["HeaderFileDirectoryListing"][i];
                        if (name.value() == del_name) {
                            config_["HeaderFileDirectoryListing"].erase(i);
                            continue;
                        }
                    }
                } else if (operation == "Exit") {
                    break;
                }
            }
        } else if (value == "SourceFileDirectoryListing") {
            vector<string> keys = {"1", "2", "3", "4"}, values = {"View source directories", "Add source directory", "Remove source directory", "Exit"};
            while (true) {
                string operation = _window.display_menu(keys, values);
                if (operation == "View source directories") {
                    vector<string> nums, list;
                    for (int i = 0; i < config_["SourceFileDirectoryListing"].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["SourceFileDirectoryListing"][i];
                        nums.push_back(name.value());
                    }
                    _window.display_menu(nums, list);
                } else if (operation == "Add source directory") {
                    string name;
                    int ret = _window.get_input(name, "Input Library name");
                    if (ret == -1 || name == "") {
                        continue;
                    }
                    config_["SourceFileDirectoryListing"].add(name);
                } else if (operation == "Remove source directory") {
                    vector<string> nums, list;
                    for (int i = 0; i < config_["SourceFileDirectoryListing"].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["SourceFileDirectoryListing"][i];
                        nums.push_back(name.value());
                    }
                    string del_name = _window.display_menu(nums, list);
                    if (del_name == "") {
                        continue;
                    }

                    string title = "Are you sure to delete ";
                    bool is_delete = _window.message(title + del_name);
                    for (int i = 0; i < config_["SourceFileDirectoryListing"].size() && is_delete; ++i) {
                        JsonString name = config_["SourceFileDirectoryListing"][i];
                        if (name.value() == del_name) {
                            config_["SourceFileDirectoryListing"].erase(i);
                            continue;
                        }
                    }
                } else if (operation == "Exit") {
                    break;
                }
            }
        } else if (value == "LibraryDirectoryListing") {
            vector<string> keys = {"1", "2", "3", "4"}, values = {"View library directories", "Add library directory", "Remove library directory", "Exit"};
            while (true) {
                string operation = _window.display_menu(keys, values);
                if (operation == "View library directories") {
                    vector<string> nums, list;
                    for (int i = 0; i < config_["LibraryDirectoryListing"].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["LibraryDirectoryListing"][i];
                        nums.push_back(name.value());
                    }
                    _window.display_menu(nums, list);
                } else if (operation == "Add library directory") {
                    string name;
                    int ret = _window.get_input(name, "Input Library name");
                    if (ret == -1 || name == "") {
                        continue;
                    }
                    config_["LibraryDirectoryListing"].add(name);
                } else if (operation == "Remove library directory") {
                    vector<string> nums, list;
                    for (int i = 0; i < config_["LibraryDirectoryListing"].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["LibraryDirectoryListing"][i];
                        nums.push_back(name.value());
                    }
                    string del_name = _window.display_menu(nums, list);
                    if (del_name == "") {
                        continue;
                    }

                    string title = "Are you sure to delete ";
                    bool is_delete = _window.message(title + del_name);
                    for (int i = 0; i < config_["LibraryDirectoryListing"].size() && is_delete; ++i) {
                        JsonString name = config_["LibraryDirectoryListing"][i];
                        if (name.value() == del_name) {
                            config_["LibraryDirectoryListing"].erase(i);
                            continue;
                        }
                    }
                } else if (operation == "Exit") {
                    break;
                }
            }
        }
    }
}


int 
Project::pull_file(void)
{
    JsonString js_value = config_["ImportAndExportDirectory"];
    string path = js_value.value();
    if (path == "") {
        return -1;
    }

    string include_path = path + "/include";
    string library_path = path + "/lib";

    string result;
    exe_shell_cmd(result, "ls ./inc");
}

int 
Project::push_file(void)
{

}