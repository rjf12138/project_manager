#include "project.h"
#include "exe_shell.h"
#include "system_utils.h"

Project::Project(void)
    :is_project_opened_(false), 
     name_("")
{
}

Project::~Project(void)
{

}

int 
Project::init(void)
{
#ifdef __RJF_LINUX__
    exe_shell_cmd(project_install_path_, "cat $HOME/.project_manager.ini");
    project_install_path_[project_install_path_.length() - 1] = '/';//将最后的换行符赋值为空
    if (access(project_install_path_.c_str(), 0) == -1) {
        LOG_GLOBAL_ERROR("Init Failed: %s is not exists.", project_install_path_.c_str());
        return -1;
    }
#endif
    ByteBuffer buffer;
    system_utils::Stream project_paths;
    string project_paths_config = project_install_path_ + "project_path.json";
    int ret = project_paths.open(project_paths_config.c_str());
    if (ret == -1) {
        LOG_GLOBAL_ERROR("Load project failed: Can't find config at %s", project_paths_config.c_str());
        return -1;
    }
    project_paths.read(buffer, project_paths.file_size());
    WeJson js_project_paths(buffer);
    JsonString js_name = js_project_paths["RecentOpenProject"]["Name"];
    JsonString js_path = js_project_paths["RecentOpenProject"]["Path"];
    
    string project_config_path = js_path.value() + "/.proj_config/project_config.json";
    if (access(project_config_path.c_str(), 0) != -1) {
        name_ = js_name.value();
        project_path_ = js_path.value();
        
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
    }

    return 0;
}

int 
Project::load_project(string project_path)
{
    bool is_new_project = false;
    ByteBuffer buffer;
    system_utils::Stream project_paths;
    string project_paths_config = project_install_path_ + "project_path.json";
    int ret = project_paths.open(project_paths_config.c_str());
    if (ret == -1) {
        LOG_GLOBAL_ERROR("Load project failed: Can't find config at %s", project_paths_config.c_str());
        return -1;
    }
    project_paths.read(buffer, project_paths.file_size());
    WeJson js_project_paths(buffer);
    
    if (project_path == "") { // 参数为空则从缓存文件中读取
        vector<string> names={"Input Project Path"}, paths = {""};
        JsonString js_name = js_project_paths["RecentOpenProject"]["Name"];
        JsonString js_path = js_project_paths["RecentOpenProject"]["Path"];
        names.push_back(js_name.value());
        paths.push_back(js_path.value());
        
        auto iter = js_project_paths["ProjectPaths"].begin();
        auto iter_end = js_project_paths["ProjectPaths"].end();
        for (; iter != iter_end; ++iter) {
            js_name = iter.second()["Name"];
            js_path = iter.second()["Path"];

            if (js_path.value() == paths[1]) { // 最近打开过的项目，已经添加进去了，不用在加了
                continue;
            }

            names.push_back(js_name.value());
            paths.push_back(js_path.value());
        }

        project_path_ = _window.display_menu(names, paths).second;
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

        JsonString name = config_["Name"];
        name_ = name.value();
    } else {
        project_path_ = project_path;
        is_new_project = true;
    }
    
    if (is_new_project == true) {
        WeJson new_project("{}");
        new_project["Name"] = name_;
        new_project["Path"] = project_path_;
        js_project_paths["ProjectPaths"].add(new_project);
    }

    js_project_paths["RecentOpenProject"]["Name"] = name_;
    js_project_paths["RecentOpenProject"]["Path"] = project_path_;

    buffer.clear();
    buffer.write_string(js_project_paths.format_json());
    project_paths.clear_file();
    project_paths.write(buffer, buffer.data_size());

    return 0;
}

int 
Project::check_project_opened(void)
{
    string config_path = project_path_ + "/.proj_config/project_config.json";
    return access(config_path.c_str(), 0);
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

    if (access(project_path_.c_str(), 0) == -1) {
        LOG_GLOBAL_ERROR("Input project path error: %s can't access!", project_path_.c_str());
        return -1;
    }

    exe_shell_cmd(project_path_, "cd %s;pwd", project_path_.c_str()); // 获取项目创建的绝对路径
    project_path_[project_path_.length() - 1] = '/'; // 最后面的换行符换成空字符
    project_path_ = project_path_ + name_; // 获取项目路径

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
    WeJson arr("[]"), obj("{}");
    config_.parse("{}");

    config_["Name"] = name_;
    config_["Path"] = project_path_;

    exe_shell_cmd(result, "uuidgen | tr -d '\n'");
    config_["UUID"] = result;

    // 根据 CompilationMethod 来选择
    arr.parse("[]");
    obj.add("release", arr);
    obj.add("debug", arr);
    config_.add("LibraryListing", obj);
    obj.clear();
    arr.clear();

    arr.parse("[]");
    obj.parse("{}");
    obj.add("release", arr);
    obj.add("debug", arr);
    config_.add("LibraryDirectoryListing", obj);
    obj.clear();
    arr.clear();

    arr.parse("[]");
    arr.add("./src/");
    arr.add("./main/");
    config_["SourceFileDirectoryListing"] = arr;
    arr.clear();

    arr.parse("[]");
    arr.add("./inc/");
    config_["HeaderFileDirectoryListing"] = arr;
    arr.clear();

    // 编译选项
    arr.parse("[]");
    obj.parse("{}");
    obj.add("CompilerName", "g++");
    obj.add("DebugOption", "-O0 -Wall -g -ggdb -std=c++11");
    obj.add("ReleaseOption", "-O2 -Wall -std=c++11");
    arr.add(obj);
    config_["ChooseCompiler"] = arr;
    obj.clear();
    arr.clear();

    config_["CompilationMethod"] = "debug";
    config_["CurrentCompiler"] = "g++";
    config_["CompilationParameters"] = "";

    // 主文件和生成文件类型
    config_["MainFileName"] = "";
    config_["GenerateFileType"] = "exe";

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
LOG_GLOBAL_DEBUG("");
    if (this->check_project_opened() == -1) {
        LOG_GLOBAL_ERROR("No project has been loaded yet");
        return -1;
    }
LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("Name");
    cfg_values.push_back(name_);
LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("Path");
    cfg_values.push_back(project_path_);
LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("UUID");
    js_value = config_["UUID"];
    cfg_values.push_back(js_value.value());
LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("Current compiler"); // 选择使用的编译器
    js_value = config_["CurrentCompiler"];
    cfg_values.push_back(js_value.value());
LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("Compilation method"); // Debug or Release
    js_value = config_["CompilationMethod"];
    cfg_values.push_back(js_value.value());
    string compile_method = js_value.value();
LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("Compilation parameters");
    js_value = config_["CompilationParameters"];
    cfg_values.push_back(js_value.value());
LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("Generate file type"); // share, static, exe
    js_value = config_["GenerateFileType"];
    cfg_values.push_back(js_value.value());
LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("Main file name"); // 主函数所在的文件名称
    js_value = config_["MainFileName"];
    cfg_values.push_back(js_value.value());
LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("Library listing"); // 库列表
    cfg_values.push_back("View");
LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("Library directory listing"); // 库目录列表
    cfg_values.push_back("View");
LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("Header directory listing"); // 头文件目录列表
    cfg_values.push_back("View");
LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("Source directory listing"); // 源文件目录列表
    cfg_values.push_back("View");
    LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("Associated project"); // 关联项目
    cfg_values.push_back("View");
LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("Export file"); // 当前项目需要共享的文件和目录
    cfg_values.push_back("View");
LOG_GLOBAL_DEBUG("");
    cfg_params.push_back("Save");   // 库和头文件导入导出目录
    cfg_values.push_back("");
LOG_GLOBAL_DEBUG("");
    while (true) {
        string value = _window.display_menu(cfg_params, cfg_values).first;
        if (value == "") {
            break;
        }

        if (value == "Current compiler") {
            while (true) {
                vector<string> compiler_nums = {"1"};
                vector<string> compiler_names = {"Add new compiler"};
                for (int i = 0; i < config_["ChooseCompiler"].size(); ++i) {
                    compiler_nums.push_back(to_string(i + 2));

                    JsonString tmp_value = config_["ChooseCompiler"][i]["CompilerName"];
                    compiler_names.push_back(tmp_value.value());
                }

                string compiler_name = _window.display_menu(compiler_nums, compiler_names).second;
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
                            if (compile_method.value() == "release") {
                                config_["CompilationParameters"] = config_["ChooseCompiler"][i]["ReleaseOption"];
                            } else {
                                config_["CompilationParameters"] = config_["ChooseCompiler"][i]["DebugOption"];
                            }
                        }
                    }
                    break;
                }
            }
        } else if (value == "Compilation method") {
            vector<string> nums = {"1", "2"};
            vector<string> compile_method = {"debug", "release"};
            string method = _window.display_menu(nums, compile_method).second;
            if (method == "") {
                continue;
            }
            config_["CompilationMethod"] = method;
            
            JsonString current_compile_name = config_["CurrentCompiler"];
            for (int i = 0; i < config_["ChooseCompiler"].size(); ++i) {
                JsonString choose_compiler_name = config_["ChooseCompiler"][i]["CompilerName"];
                if (choose_compiler_name.value() == current_compile_name.value()) {
                    if (method == "release") {
                        config_["CompilationParameters"] = config_["ChooseCompiler"][i]["ReleaseOption"];
                    } else {
                        config_["CompilationParameters"] = config_["ChooseCompiler"][i]["DebugOption"];
                    }
                }
            }
        } else if (value == "Compilation parameters") {
            JsonString param = config_["CompilationParameters"];
            string ret_param;
            int ret = _window.get_input(ret_param, "Modify compile parameters", param.value());
            if (ret != -1) {
                config_["CompilationParameters"] = ret_param;
            }
        } else if (value == "Generate file type") {
            vector<string> keys = {"1", "2", "3"};
            vector<string> values = {"exe", "static_lib", "share_lib"};

            string type = _window.display_menu(keys, values).second;
            config_["GenerateFileType"] = type;
        } else if (value == "Main file name") {
            vector<string> keys, values;
            for (int i = 0; i < config_["MainFileNameList"].size(); ++i) {
                keys.push_back(to_string(i + 1));
                JsonString main_filename = config_["MainFileNameList"][i];
                values.push_back(main_filename.value());
            }
            string main_filename = _window.display_menu(keys, values).second;
            config_["MainFileName"] = main_filename;
        } else if (value == "Library listing") {
            vector<string> keys = {"1", "2", "3", "4"}, values = {"View Libraries", "Add Library", "Remove Library", "Exit"};
            while (true) {
                string operation = _window.display_menu(keys, values).second;
                if (operation == "View Libraries") {
                    vector<string> nums, library_list;
                    for (int i = 0; i < config_["LibraryListing"][compile_method].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["LibraryListing"][compile_method][i];
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
                    for (int i = 0; i < config_["LibraryListing"][compile_method].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["LibraryListing"][compile_method][i];
                        nums.push_back(name.value());
                    }
                    string library_name = _window.display_menu(nums, library_list).second;
                    if (library_name == "") {
                        continue;
                    }

                    string title = "Are you sure to delete ";
                    bool is_delete = _window.message(title + library_name);
                    for (int i = 0; i < config_["LibraryListing"][compile_method].size() && is_delete; ++i) {
                        JsonString name = config_["LibraryListing"][compile_method][i];
                        if (name.value() == library_name) {
                            config_["LibraryListing"][compile_method].erase(i);
                            continue;
                        }
                    }
                }
            }
        } else if (value == "Header directory listing") {
            vector<string> keys = {"1", "2", "3", "4"}, values = {"View header directories", "Add header directory", "Remove header directory", "Exit"};
            while (true) {
                string operation = _window.display_menu(keys, values).second;
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
                    string del_name = _window.display_menu(nums, list).second;
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
        } else if (value == "Source directory listing") {
            vector<string> keys = {"1", "2", "3", "4"}, values = {"View source directories", "Add source directory", "Remove source directory", "Exit"};
            while (true) {
                string operation = _window.display_menu(keys, values).second;
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
                    string del_name = _window.display_menu(nums, list).second;
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
        } else if (value == "Library directory listing") {
            vector<string> keys = {"1", "2", "3", "4"}, values = {"View library directories", "Add library directory", "Remove library directory", "Exit"};
            while (true) {
                string operation = _window.display_menu(keys, values).second;
                if (operation == "View library directories") {
                    vector<string> nums, list;
                    for (int i = 0; i < config_["LibraryDirectoryListing"][compile_method].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["LibraryDirectoryListing"][compile_method][i];
                        nums.push_back(name.value());
                    }
                    _window.display_menu(nums, list);
                } else if (operation == "Add library directory") {
                    string name;
                    int ret = _window.get_input(name, "Input Library name");
                    if (ret == -1 || name == "") {
                        continue;
                    }
                    config_["Library directory listing"].add(name);
                } else if (operation == "Remove library directory") {
                    vector<string> nums, list;
                    for (int i = 0; i < config_["LibraryDirectoryListing"][compile_method].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["LibraryDirectoryListing"][compile_method][i];
                        nums.push_back(name.value());
                    }
                    string del_name = _window.display_menu(nums, list).second;
                    if (del_name == "") {
                        continue;
                    }

                    string title = "Are you sure to delete ";
                    bool is_delete = _window.message(title + del_name);
                    for (int i = 0; i < config_["LibraryDirectoryListing"][compile_method].size() && is_delete; ++i) {
                        JsonString name = config_["LibraryDirectoryListing"][compile_method][i];
                        if (name.value() == del_name) {
                            config_["LibraryDirectoryListing"][compile_method].erase(i);
                            continue;
                        }
                    }
                } else if (operation == "Exit") {
                    break;
                }
            }
        } else if (value == "Associated project") {
            vector<string> keys = {"1", "2", "3", "4"}, values = {"View associated project", "Add associated project", "Remove associated project", "Exit"};
            while (true) {
                string operation = _window.display_menu(keys, values).second;
                if (operation == "View associated project") {
                    vector<string> nums, list;
                    for (int i = 0; i < config_["AssociatedProject"].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["AssociatedProject"][i];
                        nums.push_back(name.value());
                    }
                    _window.display_menu(nums, list);
                } else if (operation == "Add associated project") {
                    string name;
                    int ret = _window.get_input(name, "Input project name");
                    if (ret == -1 || name == "") {
                        continue;
                    }
                    config_["AssociatedProject"].add(name);
                } else if (operation == "Remove associated project") {
                    vector<string> nums, list;
                    for (int i = 0; i < config_["AssociatedProject"].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["AssociatedProject"][i];
                        nums.push_back(name.value());
                    }
                    string del_name = _window.display_menu(nums, list).second;
                    if (del_name == "") {
                        continue;
                    }

                    string title = "Are you sure to delete ";
                    bool is_delete = _window.message(title + del_name);
                    for (int i = 0; i < config_["AssociatedProject"].size() && is_delete; ++i) {
                        JsonString name = config_["AssociatedProject"][i];
                        if (name.value() == del_name) {
                            config_["AssociatedProject"].erase(i);
                            continue;
                        }
                    }
                } else if (operation == "Exit") {
                    break;
                }
            }
        } else if (value == "Export file") {
            vector<string> keys = {"1", "2", "3", "4"}, values = {"View ExportFile", "Add ExportFile", "Remove ExportFile", "Exit"};
            while (true) {
                string operation = _window.display_menu(keys, values).second;
                if (operation == "View ExportFile") {
                    vector<string> nums, list;
                    for (int i = 0; i < config_["ExportFile"].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["ExportFile"][i];
                        nums.push_back(name.value());
                    }
                    _window.display_menu(nums, list);
                } else if (operation == "Add ExportFile") {
                    string name;
                    int ret = _window.get_input(name, "Input export file name");
                    if (ret == -1 || name == "") {
                        continue;
                    }
                    config_["ExportFile"].add(name);
                } else if (operation == "Remove ExportFile") {
                    vector<string> nums, list;
                    for (int i = 0; i < config_["ExportFile"].size(); ++i) {
                        nums.push_back(to_string(i+1));
                        JsonString name = config_["ExportFile"][i];
                        nums.push_back(name.value());
                    }
                    string del_name = _window.display_menu(nums, list).second;
                    if (del_name == "") {
                        continue;
                    }

                    string title = "Are you sure to delete ";
                    bool is_delete = _window.message(title + del_name);
                    for (int i = 0; i < config_["ExportFile"].size() && is_delete; ++i) {
                        JsonString name = config_["ExportFile"][i];
                        if (name.value() == del_name) {
                            config_["ExportFile"].erase(i);
                            continue;
                        }
                    }
                } else if (operation == "Exit") {
                    break;
                }
            }
        } else if (value == "Save") {
            string project_config_path = project_path_ + "/.proj_config/project_config.json";
            ByteBuffer buffer;
            buffer.write_string(config_.format_json());
            system_utils::Stream project_config;
            project_config.open(project_config_path);
            project_config.write(buffer, buffer.data_size());
            break;
        }
    }

    return 0;
}


int 
Project::pull_file(void)
{
    JsonString js_value = config_["ImportAndExportDirectory"];
    string path = js_value.value();
    if (path == "") {
        return -1;
    }

    JsonString compile_method = config_["CompilationMethod"];
#ifdef __RJF_WINDOWS__
    string include_path = path + "/project_manager/local/include";
    string library_path = path + "/project_manager/local/lib";
#else
    string include_path = path + "/project_manager/local/include";
    string library_path = path + "/project_manager/local/lib/" + compile_method.value();
#endif

    ByteBuffer buffer;
    string js_file_path = path + "/project_manager/local/ProjectAssociatedFile.json";
    system_utils::Stream file_stream;

    file_stream.open(js_file_path);
    file_stream.read(buffer, file_stream.file_size());
    WeJson js_file(buffer);

    JsonArray associate_proj_array = config_["AssociatedProject"];
    // 使用 rsync -r -u --delete ;-r 递归目录， -u 更新 --delete 如果源端没有的文件则会在目标端删除
    for (int i = 0; i < associate_proj_array.size(); ++i) {
        JsonString name = associate_proj_array[i];
        for (int j = 0; j < js_file.size(); ++j) {
            JsonObject object = js_file[i];
            JsonString proj_name = object["Name"];
            if (name.value() == proj_name.value()) {
                // 同步头文件
                JsonArray array = object["Include"];
                for (int i = 0; i < array.size(); ++i) {
                    JsonString pull_file = array[i];
                    string cmd = "rsync -r -u --delete ";
                    cmd += include_path + "/" + proj_name.value() + "/" + pull_file.value() + " ";
                    cmd += project_path_ + "/inc/" + pull_file.value();
                }

                // 同步库文件
                array = object["Library"];
                for (int i = 0; i < array.size(); ++i) {
                    JsonString pull_file = array[i];
                    string cmd = "rsync -r -u --delete ";
                    cmd += library_path + "/" + proj_name.value() + "/" + pull_file.value() + " ";
                    cmd += project_path_ + "/lib/" + compile_method.value() +"/" + pull_file.value();
                }
            }
        }
    }

    return 0;
}

int 
Project::push_file(void)
{
    JsonString js_value = config_["ImportAndExportDirectory"];
    string path = js_value.value();
    if (path == "") {
        return -1;
    }

    JsonString compile_method = config_["CompilationMethod"];
#ifdef __RJF_WINDOWS__
    string include_path = path + "/project_manager/local/include";
    string library_path = path + "/project_manager/local/lib";
#else
    string include_path = path + "/project_manager/local/include";
    string library_path = path + "/project_manager/local/lib/" + compile_method.value();
#endif

    ByteBuffer buffer;
    string js_file_path = path + "/project_manager/local/ProjectAssociatedFile.json";
    system_utils::Stream file_stream;

    file_stream.open(js_file_path);
    file_stream.read(buffer, file_stream.file_size());
    WeJson js_file(buffer);

    JsonArray push_inc_file = config_["PushFile"]["include"]; // 提交头文件
    for (int i = 0; i < push_inc_file.size(); ++i) {
        JsonString push_file = push_inc_file[i];
        string cmd = "rsync -r -u --delete ";
        cmd += project_path_ + "/inc/" + push_file.value() + " ";
        cmd += include_path + "/" + push_file.value();
    }

    JsonArray push_lib_file = config_["PushFile"]["library"];
    for (int i = 0; i < push_lib_file.size(); ++i) {
        JsonString push_file = push_inc_file[i];
        string cmd = "rsync -r -u --delete ";
        cmd += project_path_ + "/output/" + compile_method.value() + "/lib/" + push_file.value() + " ";
        cmd += library_path + "/" + push_file.value();
    }

    return 0;
}