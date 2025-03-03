#ifndef __PROJ_H__
#define __PROJ_H__
#include "config.h"

class Proj {
public:
    Proj(void);
    ~Proj(void);

    int create_project(const std::string &proj_name, const std::string &proj_path);
    int load_proj(const std::string &proj_path);

private:
    ProjConfig cfg_;
};

class ProjManager {
public:
    ProjManager(void);
    ~ProjManager(void);

    int init(void);

    int create_project(const std::string &proj_name, const std::string &proj_path);
    int load_proj(const std::string &proj_path);

private:
    std::map<std::string, Proj> projs_;
};

#endif