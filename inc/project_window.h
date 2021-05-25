#ifndef __WINDOW_H__
#define __WINDOW_H__

#include "basic_head.h"
#include "ncurses/ncurses.h"
#include "ncurses/menu.h"
#include "ncurses/form.h"

#define PrintChars  "~`!@#$%^&*()_-+={}[]\\|:;\"',<.>/? *-+"

#define ProjWin_RetEmpty    ""
#define ProjWin_InputPath   "NULL"

// 定义key
#define Keyboard_Tab        9
#define Keyboard_Enter      10
#define Keyboard_Esc        27

#define Form_MaxInput       69

class ProjectWindow {
public:
    ProjectWindow(void);
    virtual ~ProjectWindow(void);

    /// 失败返回"", 主动输入路径返回 "-Input"
    string display_menu(vector<string> proj_name, vector<string> proj_path);
    /// 获取输入
    string get_input(string title);
    // 消息提示
    bool message(string title);
private:
    void print_in_middle(WINDOW *win, int starty, int startx, int width, const char *string, chtype color);
private:
};

#endif