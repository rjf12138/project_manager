#include "proj.h"

Proj::Proj(void)
{}

Proj::~Proj(void)
{}

int 
Proj::create_proj_dir(const std::string &proj_name, const std::string &proj_path)
{
    return dir_.init_proj_dir(proj_name, proj_path);
}

int 
Proj::load_proj_dir(const std::string &proj_path)
{
    return this->dir_.load_proj_dir(proj_path);
}