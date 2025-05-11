#include "proj_config.h"
#include "system/system.h"

ProjConfigManager::ProjConfigManager(void)
{}

ProjConfigManager::~ProjConfigManager(void)
{}

int 
ProjConfigManager::init_cfg(void)
{
    // cfg todo
    this->save_cfg(cfg_);
}

int 
ProjConfigManager::load_cfg(const std::string &cfg_path)
{
    os::File file;
    file.open(cfg_path);

    basic::ByteBuffer buffer;
    file.read(buffer, file.file_size());

    return this->convert_json_to_proj_cfg(buffer.str(), cfg_);
}

ProjConfig 
ProjConfigManager::get_cfg(void)
{
    return cfg_;
}

void 
ProjConfigManager::save_cfg(ProjConfig cfg)
{
    basic::WeJson js_cfg;
    this->convert_proj_cfg_to_json(cfg, js_cfg);

    os::File file;
    file.open(cfg.proj_path);

    basic::ByteBuffer buffer = js_cfg.to_string();
    file.write(buffer, buffer.data_size());
}

int 
ProjConfigManager::convert_proj_cfg_to_json(ProjConfig cfg, basic::WeJson &cfg_json)
{
    cfg_json.parse("{}");
    basic::JsonObject &cfg_object = cfg_json.get_object();
    cfg_object["build_method"] = cfg.build_method;
    cfg_object["generate_file_type"] = cfg.generate_file_type;
    cfg_object["main_file"] = cfg.main_file;
    cfg_object["proj_name"] = cfg.proj_name;
    cfg_object["proj_path"] = cfg.proj_path;
    cfg_object["proj_run_args"] = cfg.proj_run_args;
    cfg_object["proj_uuid"] = cfg.proj_uuid;

    basic::JsonObject proj_depend;
    basic::JsonArray js_tmp_arr;

    for (ssize_t i = 0; i < cfg.proj_depend.debug_lib.size(); ++i) {
        js_tmp_arr.add(cfg.proj_depend.debug_lib[i]);
    }
    proj_depend["debug_lib"] = js_tmp_arr;

    js_tmp_arr.clear();
    for (ssize_t i = 0; i < cfg.proj_depend.debug_lib_dir.size(); ++i) {
        js_tmp_arr.add(cfg.proj_depend.debug_lib_dir[i]);
    }
    proj_depend["debug_lib_dir"] = js_tmp_arr;

    js_tmp_arr.clear();
    for (ssize_t i = 0; i < cfg.proj_depend.header_file_dirs.size(); ++i) {
        js_tmp_arr.add(cfg.proj_depend.header_file_dirs[i]);
    }
    proj_depend["header_file_dirs"] = js_tmp_arr;

    js_tmp_arr.clear();
    for (ssize_t i = 0; i < cfg.proj_depend.release_lib.size(); ++i) {
        js_tmp_arr.add(cfg.proj_depend.release_lib[i]);
    }
    proj_depend["release_lib"] = js_tmp_arr;

    js_tmp_arr.clear();
    for (ssize_t i = 0; i < cfg.proj_depend.release_lib_dir.size(); ++i) {
        js_tmp_arr.add(cfg.proj_depend.release_lib_dir[i]);
    }
    proj_depend["release_lib_dir"] = js_tmp_arr;
    
    js_tmp_arr.clear();
    for (ssize_t i = 0; i < cfg.proj_depend.src_file_dirs.size(); ++i) {
        js_tmp_arr.add(cfg.proj_depend.src_file_dirs[i]);
    }
    proj_depend["src_file_dirs"] = js_tmp_arr;
    cfg_json["proj_depend"] = proj_depend;

    js_tmp_arr.clear();
    basic::JsonObject js_tmp_obj;
    for (auto iter = cfg.compiler_cfg.begin(); iter != cfg.compiler_cfg.end(); ++iter) {
        js_tmp_obj["name"] = iter->first;
        js_tmp_obj["compiler_path"] = iter->second.compiler_path;
        js_tmp_obj["compiler_option"] = iter->second.compile_option;
        js_tmp_arr.add(js_tmp_obj);
    }
    cfg_json["compiler_cfg"] = js_tmp_arr;

    return 0;
}

int 
ProjConfigManager::convert_json_to_proj_cfg(const std::string &str_cfg_json, ProjConfig &cfg)
{
    basic::WeJson cfg_json;
    cfg_json.parse(str_cfg_json.c_str());
    basic::JsonObject &cfg_object = cfg_json.get_object();
    cfg.build_method = cfg_object["build_method"].to_string();
    cfg.generate_file_type = cfg_object["generate_file_type"].to_string();
    cfg.main_file = cfg_object["main_file"].to_string();
    cfg.proj_name = cfg_object["proj_name"].to_string();
    cfg.proj_path = cfg_object["proj_path"].to_string();
    cfg.proj_run_args = cfg_object["proj_run_args"].to_string();
    cfg.proj_uuid = cfg_object["proj_uuid"].to_string();

    basic::JsonArray js_tmp_arr;
    basic::JsonObject proj_depend = cfg_object["proj_depend"];

    js_tmp_arr = proj_depend["debug_lib"];
    for (auto iter = js_tmp_arr.begin(); iter != js_tmp_arr.end(); ++iter) {
        cfg.proj_depend.debug_lib.push_back(iter->to_string());
    }

    js_tmp_arr = proj_depend["debug_lib_dir"];
    for (auto iter = js_tmp_arr.begin(); iter != js_tmp_arr.end(); ++iter) {
        cfg.proj_depend.debug_lib_dir.push_back(iter->to_string());
    }

    js_tmp_arr = proj_depend["header_file_dirs"];
    for (auto iter = js_tmp_arr.begin(); iter != js_tmp_arr.end(); ++iter) {
        cfg.proj_depend.header_file_dirs.push_back(iter->to_string());
    }

    js_tmp_arr = proj_depend["release_lib"];
    for (auto iter = js_tmp_arr.begin(); iter != js_tmp_arr.end(); ++iter) {
        cfg.proj_depend.release_lib.push_back(iter->to_string());
    }

    js_tmp_arr = proj_depend["release_lib_dir"];
    for (auto iter = js_tmp_arr.begin(); iter != js_tmp_arr.end(); ++iter) {
        cfg.proj_depend.release_lib_dir.push_back(iter->to_string());
    }

    js_tmp_arr = proj_depend["src_file_dirs"];
    for (auto iter = js_tmp_arr.begin(); iter != js_tmp_arr.end(); ++iter) {
        cfg.proj_depend.src_file_dirs.push_back(iter->to_string());
    }

    js_tmp_arr = cfg_object["compiler_cfg"];
    for (auto iter = js_tmp_arr.begin(); iter != js_tmp_arr.end(); ++iter) {
        basic::JsonObject js_tmp_obj = *iter;
        cfg.compiler_cfg[js_tmp_obj["name"].to_string()].compile_option = js_tmp_obj["compiler_path"].to_string();
        cfg.compiler_cfg[js_tmp_obj["name"].to_string()].compile_option = js_tmp_obj["compiler_option"].to_string();
    }

    return 0;
}