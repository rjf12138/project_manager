#ifndef __WINDOW_H__
#define __WINDOW_H__

// doc: https://www.jb51.net/article/155963.htm

#include "basic_head.h"
#include "ncurses/ncurses.h"
#include "ncurses/menu.h"

class ProjectWindow {
public:
    ProjectWindow(void);
    virtual ~ProjectWindow(void);

    string menu_manager(vector<string> print_array, int menu_display_num = 12);
    string display_menu(vector<string> print_array, std::size_t start, std::size_t tail);
private:
};

#endif