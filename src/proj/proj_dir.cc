#include "proj_dir.h"
#include "basic/process_control.h"

ProjDir::ProjDir(void) {}
ProjDir::~ProjDir(void) {}

// 初始化项目目录
int 
ProjDir::init_proj_dir(const std::string &proj_name, const std::string &proj_generate_path)
{
    os::Directory generate_path_dir;
    RETURN_FUNC_LT(generate_path_dir.open_dir(proj_generate_path), 0, -1);

    std::string proj_path = generate_path_dir.get_abs_path(proj_generate_path) + "/" +proj_name;
    proj_dir_info_.proj_path = proj_path;
    proj_dir_info_.program_config_path = proj_path + "/config";
    proj_dir_info_.program_debug_lib_path = proj_path + "/lib/debug";
    proj_dir_info_.program_debug_output_bin_path = proj_path + "/output/debug/bin";
    proj_dir_info_.program_debug_output_lib_path = proj_path + "/output/debug/lib";
    proj_dir_info_.program_doc_path = proj_path + "/doc";
    proj_dir_info_.program_extern_inc_path = proj_path + "/extern_inc";
    proj_dir_info_.program_inc_path = proj_path + "/inc";
    proj_dir_info_.program_main_path = proj_path + "/main";
    proj_dir_info_.program_release_lib_path = proj_path + "/lib/release";
    proj_dir_info_.program_release_output_bin_path = proj_path + "/output/release/bin";
    proj_dir_info_.program_release_output_lib_path = proj_path + "/output/release/lib";
    
    return this->create_dir_from_dir_info(proj_dir_info_);
}

int 
ProjDir::load_proj_dir(const std::string &proj_path)
{
    return this->generate_dir_info_from_dir(proj_path);
}

ProjDirInfo 
ProjDir::get_proj_dir_info(void)
{
    return proj_dir_info_;
}

int 
ProjDir::create_dir_from_dir_info(ProjDirInfo dir_info)
{
    os::Directory proj_path_dir;
    if (proj_path_dir.exist(dir_info.proj_path, true) == true) {
        LOG_GLOBAL_WARN("%s is exist", dir_info.proj_path.c_str());
        return -1;
    }
    
    RETURN_FUNC_LT(proj_path_dir.create(dir_info.proj_path, os::EFileType_Dir), 0, -1);
    RETURN_FUNC_LT(proj_path_dir.open_dir(dir_info.proj_path), 0, -1);

    RETURN_FUNC_LT(proj_path_dir.create(dir_info.proj_path, os::EFileType_Dir), 0, -1);

    RETURN_FUNC_LT(proj_path_dir.create(dir_info.program_config_path, os::EFileType_Dir), 0, -1);
    RETURN_FUNC_LT(proj_path_dir.create(dir_info.program_doc_path, os::EFileType_Dir), 0, -1);
    RETURN_FUNC_LT(proj_path_dir.create(dir_info.program_extern_inc_path, os::EFileType_Dir), 0, -1);
    RETURN_FUNC_LT(proj_path_dir.create(dir_info.program_inc_path, os::EFileType_Dir), 0, -1);
    RETURN_FUNC_LT(proj_path_dir.create(dir_info.program_release_lib_path, os::EFileType_Dir), 0, -1);
    RETURN_FUNC_LT(proj_path_dir.create(dir_info.program_debug_lib_path, os::EFileType_Dir), 0, -1);
    RETURN_FUNC_LT(proj_path_dir.create(dir_info.program_main_path, os::EFileType_Dir), 0, -1);
    RETURN_FUNC_LT(proj_path_dir.create(dir_info.program_release_output_bin_path, os::EFileType_Dir), 0, -1);
    RETURN_FUNC_LT(proj_path_dir.create(dir_info.program_release_output_lib_path, os::EFileType_Dir), 0, -1);
    RETURN_FUNC_LT(proj_path_dir.create(dir_info.program_debug_output_bin_path, os::EFileType_Dir), 0, -1);
    RETURN_FUNC_LT(proj_path_dir.create(dir_info.program_debug_output_lib_path, os::EFileType_Dir), 0, -1);

    RETURN_FUNC_LT(proj_path_dir.create(dir_info.proj_path + "/src", os::EFileType_Dir), 0, -1);

    return 0;
}

int 
ProjDir::generate_dir_info_from_dir(const std::string &proj_path)
{
    os::Directory proj_path_dir;
    if (proj_path_dir.exist(proj_path, true) == false) {
        LOG_GLOBAL_WARN("%s is not exist", proj_path.c_str());
        return -1;
    }

    RETURN_FUNC_LT(proj_path_dir.open_dir(proj_path), 0, -1);

    std::map<std::string, os::SFileType> proj_dir_file_list = proj_path_dir.file_list();
    
    
    proj_dir_info_.proj_path = proj_path;
    proj_dir_info_.program_config_path = proj_dir_file_list[".vscode"].abs_path_;
    proj_dir_info_.program_doc_path = proj_dir_file_list["doc"].abs_path_;

    proj_dir_info_.program_extern_inc_path = proj_dir_file_list["extern_inc"].abs_path_;
    proj_dir_info_.program_inc_path = proj_dir_file_list["inc"].abs_path_;

    proj_dir_info_.program_release_lib_path = proj_dir_file_list["lib"].abs_path_ + "/release";
    proj_dir_info_.program_debug_lib_path = proj_dir_file_list["lib"].abs_path_ + "/debug";

    proj_dir_info_.program_main_path = proj_dir_file_list["main"].abs_path_;

    proj_dir_info_.program_release_output_bin_path = proj_dir_file_list["output"].abs_path_ + "/release/bin";
    proj_dir_info_.program_release_output_lib_path = proj_dir_file_list["output"].abs_path_ + "/release/lib";

    proj_dir_info_.program_debug_output_bin_path = proj_dir_file_list["output"].abs_path_ + "/debug/bin";
    proj_dir_info_.program_debug_output_lib_path = proj_dir_file_list["output"].abs_path_ + "/debug/lib";

    return 0;
}