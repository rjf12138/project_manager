#ifndef __WINDOW_H__
#define __WINDOW_H__

#include "basic_head.h"
#include "ncurses/ncurses.h"
#include "ncurses/menu.h"

#define ProjWin_RetEmpty    ""
#define ProjWin_InputPath   "Input Project Path"

class ProjectWindow {
public:
    ProjectWindow(void);
    virtual ~ProjectWindow(void);

    /// 失败返回"", 主动输入路径返回 "-Input"
    string display_menu(vector<string> proj_name, vector<string> proj_path);
private:
    void print_in_middle(WINDOW *win, int starty, int startx, int width, const char *string, chtype color);
private:
};

#endif