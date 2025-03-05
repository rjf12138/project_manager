#ifndef __PROJ_DIR_H__
#define __PROJ_DIR_H__
#include "basic/basic_head.h"
#include "system/system.h"

struct ProjDirInfo {
    std::string proj_path;                              // 项目目录
        
    std::string proj_confg_path;                        // 获取项目配置文件所在目录

    std::string program_config_path;                    // 程序配置文件目录
    std::string program_doc_path;                       // 程序文档目录
    std::string program_extern_inc_path;                // 程序第三方库头文件目录
    std::string program_inc_path;                       // 程序对外开放头文件目录
    std::string program_release_lib_path;               // 程序release库目录
    std::string program_debug_lib_path;                 // 程序debug库目录
    std::string program_main_path;                      // 程序主函数目录
    std::string program_release_output_bin_path;        // 程序生成release版执行文件目录
    std::string program_release_output_lib_path;        // 程序生成release版库文件目录
    std::string program_debug_output_bin_path;          // 程序生成debug版执行文件目录
    std::string program_debug_output_lib_path;          // 程序生成debug版库文件目录
};

class ProjDir {
public:
    ProjDir(void);
    ~ProjDir(void);

    // 初始化项目目录
    int init_proj_dir(const std::string &proj_name, const std::string &proj_generate_path);
    // 加载项目目录
    int load_proj_dir(const std::string &proj_path);

    // 获取目录信息
    ProjDirInfo get_proj_dir_info(void);
    // 根据当前目录读取子目录列表
    std::vector<std::string> proj_dir_list(const std::string &parent_dir);

private:
    // 根据当前目录生成项目目录信息
    int create_dir_from_dir_info(ProjDirInfo dir_info);
    // 读取项目真实目录生成项目信息
    int generate_dir_info_from_dir(const std::string &proj_path);

private:
    ProjDirInfo proj_dir_info_;
    os::Directory dir_;
};

#endif