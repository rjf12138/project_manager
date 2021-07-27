#include "cmake.h"
#include "system_utils.h"
#include "exe_shell.h"
#include <dirent.h>

CMake::CMake(Project &proj)
{
    name_ = proj.get_project_name();
    proj_config_ = proj.get_project_config();
    project_install_path_ = proj.get_project_install_path();
    cmake_bin_path_ = proj.get_project_install_path() + "/program/bin/cmake/cmake";
    JsonString path = (*proj_config_)["Path"];
    project_path_ = path.value().c_str();

    char buffer[1024] = "";
    getcwd(buffer, 1024);
    old_workspace_path_ = buffer;
    chdir(project_path_.c_str());
}

CMake::~CMake(void)
{
    chdir(old_workspace_path_.c_str());
}


int CMake::create_top_level_cmakefile(void)
{
    if (proj_config_ == nullptr) {
        return -1;
    }
    JsonString generate_file_type = (*proj_config_)["GenerateFileType"];
    JsonString compile_method = (*proj_config_)["CompilationMethod"];
    JsonArray inc_dirs = (*proj_config_)["HeaderFileDirectoryListing"];
    JsonArray src_dirs = (*proj_config_)["SourceFileDirectoryListing"];
    JsonArray lib_dirs = (*proj_config_)["LibraryDirectoryListing"][compile_method.value()];
    JsonArray libs = (*proj_config_)["LibraryListing"][compile_method.value()];

    ostringstream cmakefile_stream;
    cmakefile_stream << "cmake_minimum_required(VERSION 3.10)" << std::endl << std::endl;
    cmakefile_stream << "# 设置项目名称" << std::endl;
    cmakefile_stream << "project(" << name_ << ")" << std::endl << std::endl;

    cmakefile_stream << "set(CMAKE_DEBUG_POSTFIX d)" << std::endl << std::endl;
    cmakefile_stream << "# 指定c++标准: 默认是 c++11" << std::endl;
    cmakefile_stream << "add_library(" << name_ << "_compiler_flags INTERFACE)" << std::endl;
    cmakefile_stream << "target_compile_features(" << name_ << "_compiler_flags INTERFACE cxx_std_11)" << std::endl;

    cmakefile_stream << "# 设置 c++ 警告标志" << std::endl;
    cmakefile_stream << "set(gcc_like_cxx \"$<COMPILE_LANG_AND_ID:CXX,ARMClang,AppleClang,Clang,GNU>\")" << std::endl;
    cmakefile_stream << "set(msvc_cxx \"$<COMPILE_LANG_AND_ID:CXX,MSVC>\")" << std::endl << std::endl;
    cmakefile_stream << "target_compile_options("<< name_ << "_compiler_flags INTERFACE" << std::endl;
    cmakefile_stream << "\t\t$<${gcc_like_cxx}:$<BUILD_INTERFACE:-Wall;-Wextra;-Wshadow;-Wformat=2;-Wunused>>" << std::endl;
    cmakefile_stream << "\t\t$<${msvc_cxx}:$<BUILD_INTERFACE:-W3>>)" << std::endl << std::endl;

    cmakefile_stream << "# 设置生成文件输出路径" << std::endl;
    if (generate_file_type.value() == "exe") {
        cmakefile_stream << "set(EXECUTABLE_OUTPUT_PATH, ./output/" << compile_method.value() << "/bin)" << std::endl;
    } else {
        cmakefile_stream << "set(LIBRARY_OUTPUT_PATH, ./output/" << compile_method.value() << "/lib)" << std::endl;
    }

    // 定义生成文件类型
    if (generate_file_type.value() == "exe") {
        JsonString main_file = (*proj_config_)["MainFileName"];
        if (main_file.value() == "") {
            LOG_GLOBAL_ERROR("Can't find program entry file!");
            return -1;
        }
        cmakefile_stream << "add_executable(" << name_ <<  " ./main/" << main_file.value() << ")" << std::endl;
    } else if (generate_file_type.value() == "static_lib") {
        cmakefile_stream << "add_library(" << name_ <<  " STATIC \"\")" << std::endl;
    } else if (generate_file_type.value() == "share_lib") {
        cmakefile_stream << "add_library(" << name_ <<  " SHARED \"\")" << std::endl;
    } else {
        LOG_GLOBAL_ERROR("Unknow File Type!");
        return -1;
    }
    cmakefile_stream << std::endl;

    // 添加头文件目录
    cmakefile_stream << "target_include_directories(" << name_ << " PUBLIC ./inc/)" << std::endl;
    cmakefile_stream << "target_include_directories(" << name_ << " PRIVATE ./extern_inc/)" << std::endl;
    for (int i = 0; i < inc_dirs.size(); ++i) {
        JsonString target_path = inc_dirs[i];
        cmakefile_stream << "target_include_directories(" << name_ << " PRIVATE " << target_path.value() << ")" << std::endl;
    }
    cmakefile_stream << std::endl;

    // 添加库目录
    cmakefile_stream << "target_link_directories(" << name_ << " PRIVATE ./lib/" << compile_method.value() << "/)" << std::endl;
    for (int i = 0; i < lib_dirs.size(); ++i) {
        JsonString target_path = lib_dirs[i];
        cmakefile_stream << "target_link_directories(" << name_ << " PRIVATE " << target_path.value() << ")" << std::endl;
    }
    cmakefile_stream << std::endl;

    // 添加第三方库
    for (int i = 0; i < libs.size(); ++i) {
        JsonString target_lib = libs[i];
        cmakefile_stream << "target_link_libraries(" << name_ << " PRIVATE " << target_lib.value() << ")" << std::endl;
    }
    
    // 使用module include加载不同的CMakeLists.txt文件
    for (int i = 0; i < src_dirs.size(); ++i) {
        JsonString target_path = src_dirs[i];
        cmakefile_stream << "include(" << target_path.value() << "/CMakeLists.txt)" << std::endl;
        create_sub_cmakefile(target_path.value(), name_);
    }
    cmakefile_stream << std::endl;
    
    // 设置安装路径
    // create_install_env()// 在安装目录中创建相关文件，修改 ProjectAssociatedFile.json
    // 安装当前项目相关头文件
    // string result;
    // string cmd = "ls inc | sed 's/ /\\n/g' | wc -w";
    // exe_shell_cmd(result, cmd.c_str());
    // if (stoi(result) > 0) {
    //     string cmd = "ls inc | sed 's/ /\\n/g'";
    //     exe_shell_cmd(result, cmd.c_str());
    //     cmakefile_stream << "install(FILES \n" << result << " DESTINATION \n\t\t"  << name_ << "/)" << std::endl; // 在cmake --install --prefix= 设置name_所在目录
    // }
    // cmakefile_stream << "install(TARGETS " << name_ << " DESTINATION lib)" << std::endl << std::endl;

    ByteBuffer buffer;
    buffer.write_string(cmakefile_stream.str());
    system_utils::Stream top_cmakefile;
    top_cmakefile.open("./CMakeLists.txt");
    top_cmakefile.clear_file();
    top_cmakefile.write(buffer, buffer.data_size());

    return 0;
}

int CMake::create_sub_cmakefile(string sub_module_path, string project_name)
{
    chdir(sub_module_path.c_str());// 切换到子模块目录下

    string sub_module_abs_path = "${CMAKE_CURRENT_SOURCE_DIR}/";
    sub_module_abs_path += sub_module_path;

    ostringstream cmakefile_stream;
    cmakefile_stream << "project(" << name_ <<")" << std::endl << std::endl;
    cmakefile_stream << "target_include_directories(" << project_name << " PRIVATE " << sub_module_abs_path << ")" << std::endl << std::endl;

    string result;
    string cmd = "find ./ -name \"*.cc\" | wc -w";
    exe_shell_cmd(result, cmd.c_str());
    if (stoi(result) > 0) {
        string cmd = "find ./ -name \"*.cc\"";
        exe_shell_cmd(result, cmd.c_str());
        ByteBuffer buff(result);
        vector<ByteBuffer> sources = buff.split(ByteBuffer("\n"));
        for (std::size_t i = 0;i < sources.size(); ++i) {
            cmakefile_stream << "target_sources(" << name_ << " PRIVATE \n\t\t" << sub_module_abs_path << "/" << sources[i].str() << ")" << std::endl; 
        }   
    }

    ByteBuffer buffer;
    buffer.write_string(cmakefile_stream.str());
    system_utils::Stream sub_cmakefile;
    sub_cmakefile.open("./CMakeLists.txt");
    sub_cmakefile.write(buffer, buffer.data_size());

    chdir(project_path_.c_str()); // 切回项目目录下
    return 0;
}

int CMake::clean_project(void)
{
    if (access("./boot/vmlinuz", 0) == F_OK){
        LOG_GLOBAL_ERROR("Current dir is root.");
        return -1;
    }

    string result;
    exe_shell_cmd_to_stdin("rm -rf ./output/release/bin/*");
    exe_shell_cmd_to_stdin("rm -rf ./output/release/lib/*");
    exe_shell_cmd_to_stdin("rm -rf ./output/debug/bin/*");
    exe_shell_cmd_to_stdin("rm -rf ./output/debug/lib/*");
    exe_shell_cmd_to_stdin("rm -rf ./build");

    return 0;
}

int CMake::build_project(bool rebuild)
{
    if (access("./boot/vmlinuz", 0) == F_OK){ // 检查是不是在根目录
        LOG_GLOBAL_ERROR("Current dir is root.");
        return -1;
    }

    if (rebuild == true) {
        clean_project();
    }

    string result;
    if (access("./build", 0) != F_OK) {
        system("mkdir ./build");
    }
    chdir("./build");
    // 编译前需要处理的事
    system("../.proj_config/exec_before_compile.sh");

    JsonString compile_method = (*proj_config_)["CompilationMethod"];
    if (compile_method.value() == "release") {
        system("cmake -DCMAKE_BUILD_TYPE=Release ..");
    } else {
        system("cmake -DCMAKE_BUILD_TYPE=Debug ..");
    }
    system("cmake --build .");

    chdir(project_path_.c_str()); // 切回项目目录下
    
    JsonString generate_file = (*proj_config_)["GenerateFileType"];
    string generate_file_name = "lib";
    if (generate_file == "share_lib") {
        if (compile_method.value() == "debug") {
            generate_file_name += name_ + "d.so";
        } else {
            generate_file_name += name_ + ".so";
        }
    } else if (generate_file == "static_lib") {
        if (compile_method.value() == "debug") {
            generate_file_name += name_ + "d.a";
        } else {
            generate_file_name += name_ + ".a";
        }
    } else {
        generate_file_name = name_;
    }
    exe_shell_cmd(result, "cp -f ./build/%s ./output/%s/%s/", 
            generate_file_name.c_str(), 
            compile_method.value().c_str(),
            generate_file.value() == "exe" ? "bin" : "lib");
    cerr << result;
    // 重新构建就重新拷贝配置文件
    if (rebuild == true) {
        if (compile_method.value() == "release") {
            system("cp -rf ./config/ ./output/release/bin");
        } else {
            system("cp -rf ./config/ ./output/debug/bin");
        }
    }
    // 编译结束需要处理的事
    system("./.proj_config/exec_after_compile.sh");

    return 0;
}

int CMake::create_install_env(void)
{
    JsonString generate_file = (*proj_config_)["GenerateFileType"];
    JsonString compile_method = (*proj_config_)["CompilationMethod"];

    string include_path = "./local/include/";
    include_path.append(name_);

    string lib_path = "./local/lib/";
    lib_path += compile_method.value() + "/" + name_ + "/linux";
    // 获取头文件
    string result;
    exe_shell_cmd(result, "ls inc | sed 's/ /\\n/g'");
    ByteBuffer buffer(result);
    vector<ByteBuffer> headers = buffer.split(ByteBuffer("\n"));

    chdir(project_install_path_.c_str());
    if (access(include_path.c_str(), 0) == -1) {
        exe_shell_cmd_to_stdin("mkdir -p %s", include_path.c_str());
    }

    if (access(lib_path.c_str(), 0) == -1) {
        exe_shell_cmd_to_stdin("mkdir -p %s", lib_path.c_str());
    }
    
    if (access("./local/ProjectAssociatedFile.json", 0) == -1) {
        exe_shell_cmd_to_stdin("echo [] > ./local/ProjectAssociatedFile.json");
    }

    system_utils::Stream config_file;
    config_file.open("./local/ProjectAssociatedFile.json");
    ByteBuffer config_data;
    config_file.read(config_data, config_file.file_size());

    WeJson obj("{}"), arr("[]");
    obj.add("Name", name_);
    for (std::size_t i = 0; i < headers.size(); ++i) {
        arr.add(headers[i].str());
    }
    obj.add("Include", arr);

    // 获取生成的库文件
    arr.clear();
    arr.parse("[]");
    string lib_name = "lib";
    if (generate_file == "share_lib") {
        if (compile_method.value() == "debug") {
            lib_name += name_ + "d.so";
        } else {
            lib_name += name_ + ".so";
        }
    } else {
        if (compile_method.value() == "debug") {
            lib_name += name_ + "d.a";
        } else {
            lib_name += name_ + ".a";
        }
    }
    arr.add(lib_name);
    obj.add("Library", arr);

    WeJson js_config(config_data);
    for (int i = 0; i < js_config.size(); ++i) {
        if (js_config[i]["Name"] == name_) {
            js_config.erase(i);
        }
    }
    js_config.add(obj);

    config_file.clear_file();
    config_data.clear();
    config_data.write_string(js_config.format_json());
    config_file.write(config_data, config_data.data_size());

    chdir(project_path_.c_str());

    return 0;
}