#ifndef __PROJECT_H__
#define __PROJECT_H__

#include "basic_head.h"

class Project {
public:
    Project(string name, string path);
    ~Project(void);

    int create_project_dir(void);
    int generate_config(void);
private:
    string name_;
    string path_;
};

#endif