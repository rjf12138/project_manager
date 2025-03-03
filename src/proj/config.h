#ifndef __CONFIG_H__
#define __CONFIG_H__
#include "basic_head.h"

// 编译器配置
struct CompilerCfg {
    std::string compiler_path;      // 编译器路径
    std::string compile_option;     // 编译选项
};

// 项目依赖项，编译时需要加进去的目录
struct ProjectDependencies {
    std::vector<std::string> header_file_dirs;  // 头文件目录
    std::vector<std::string> src_file_dirs;     // 源文件目录

    std::vector<std::string> debug_lib_dir;     // debug版本库路径
    std::vector<std::string> release_lib_dir;   // release版本库路径

    std::vector<std::string> debug_lib;         // debug依赖库名称
    std::vector<std::string> release_lib;       // release依赖库名称
};

struct ProjConfig {
    std::string proj_name;          // 项目名称
    std::string proj_path;          // 项目所在路径
    std::string proj_uuid;          // 项目uuid

    std::string main_file;          // main文件名称
    std::string proj_run_args;      // 程序运行命令
    std::string generate_file_type; // 编译生成的文件类型：exe, static, share
    std::string build_method;       // 构建类型，release，debug

    ProjectDependencies proj_depend; // 项目依赖项
    std::map<std::string, CompilerCfg> compiler_cfg; // 编译器选项
};

class ProjConfigManager {
public:
    ProjConfigManager(void);
    ~ProjConfigManager(void);

    int init_cfg(void);
    int load_cfg(const std::string &cfg_path);
    
    ProjConfig get_proj_cfg(void);
    void save_proj_cfg(ProjConfig cfg);

private:
    ProjConfig cfg_;
};

#endif