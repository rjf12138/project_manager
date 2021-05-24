#include "project_window.h"

vector<string> proj_name = {
                        "Choice 3",
                        "Choice 4",
                        "Choice 5",
                        "Choice 6",
                        "Choice 7",
                        "Choice 8",
                        "Choice 9",
                        "Choice 10",
                        "Choice 11",
                        "Choice 12",
                        "Choice 13",
                        "Choice 14",
                        "Choice 15",
                        "Choice 16",
                        "Choice 17",
                  };

vector<string> proj_path = {
                        "/home/ruanjian/project_manager/doc/Choice 3",
                        "/home/ruanjian/Choice 4",
                        "/home/ruanjian/workspace/project_pre/project_manager/doc/Choice 5",
                        "/home/ruanjian/workspace/project_pre/project_manager/doc/Choice 6",
                        "/home/ruanjian/workspace/project_pre/project_manager/doc/Choice 7",
                        "/home/ruanjian/workspace/project_pre/project_manager/doc/Choice 8",
                        "/home/ruanjian/workspace/project_pre/project_manager/doc/Choice 9",
                        "/home/ruanjian/workspace/project_pre/project_manager/doc/Choice 10",
                        "/home/ruanjian/workspace/project_pre/project_manager/doc/Choice 11",
                        "/home/ruanjian/workspace/project_pre/project_manager/doc/Choice 12",
                        "/home/ruanjian/workspace/project_pre/project_manager/doc/Choice 13",
                        "/home/ruanjian/workspace/project_pre/project_manager/doc/Choice 14",
                        "/home/ruanjian/workspace/project_pre/project_manager/doc/Choice 15",
                        "/home/ruanjian/workspace/project_pre/project_manager/doc/Choice 16",
                        "/home/ruanjian/workspace/project_pre/project_manager/doc/Choice 17",
                  };

int main(void)
{
    ProjectWindow pw;
    // string ret = pw.display_menu(proj_name, proj_path);
    string ret = pw.get_input("This is a test input.");
    cout << "ret: " << ret << " Size: " << ret.size() << endl;
    return 0;
}