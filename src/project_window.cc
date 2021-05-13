#include "project_window.h"


ProjectWindow::ProjectWindow(void)
{
    
}

ProjectWindow::~ProjectWindow(void)
{
    
}

string 
ProjectWindow::menu_manager(vector<string> print_array, int menu_display_num)
{
    std::size_t menu_page = print_array.size() / menu_display_num;
    menu_page = (print_array.size() % menu_display_num == 0 ? menu_page : menu_page + 1);
    
    return "";
}

string 
ProjectWindow::display_menu(vector<string> print_array, std::size_t start, std::size_t tail)
{
    if (start > tail) {
        return "";
    }

    if (start < 0 || start >= print_array.size()) {
        return "";
    }

    if (tail < 0 || tail >= print_array.size()) {
        return "";
    }

    initscr();  // 以 curses 模式初始化终端
    cbreak();   // 当缓存中有数据可读时直接返回而不是等到换行符或是缓冲满了才返回
    noecho();   // 输入字符不回显
    keypad(stdscr, TRUE); // 支持读取功能键像 F1, F2, arrow keys etc

    std::size_t display_num = tail - start + 4;
    ITEM** items = new ITEM*[display_num];
    items[0] = new_item("1", "Input project path");
    for (std::size_t i = start; i <= tail; ++i) {
        std::size_t index = i - start + 1;
        cerr << index << endl;
        items[index] = new_item(to_string(index).c_str(), print_array[i].c_str());
    }
    MENU *project_menu = new_menu(items);

    WINDOW *menu_window = newwin(23, 79, 0, 0);
    keypad(menu_window, TRUE);
    
    /* Set main window and sub window */
    set_menu_win(project_menu, menu_window);
    set_menu_sub(project_menu, derwin(menu_window, 20, 38, 3, 1));
    set_menu_format(project_menu, 15, 1);

    /* Set menu mark to the string " * " */
    set_menu_mark(project_menu, " > ");

    /* Print a border around the main window and print a title */
    box(menu_window, 0, 0);
    this->print_in_middle(menu_window, 1, 0, 79, "Project Path Menu", COLOR_PAIR(1));
    mvwaddch(menu_window, 2, 0, ACS_LTEE);
    mvwhline(menu_window, 2, 1, ACS_HLINE, 77);
    mvwaddch(menu_window, 2, 78, ACS_RTEE);

    mvprintw(LINES - 1, 0, "Press q to Exit");
    post_menu(project_menu);
    wrefresh(menu_window);

    int ch;
    while((ch = wgetch(menu_window)) != 'q')
    {   switch(ch)
        {   
            case KEY_DOWN: {
                menu_driver(project_menu, REQ_DOWN_ITEM);
            } break;
            case KEY_UP: {
                menu_driver(project_menu, REQ_UP_ITEM);
            } break;
            case KEY_NPAGE: {
                menu_driver(project_menu, REQ_SCR_DPAGE);
            } break;
            case KEY_PPAGE: {
                menu_driver(project_menu, REQ_SCR_UPAGE);
            } break;
        }
        wrefresh(menu_window);
    }       

    free_menu(project_menu);
    for (std::size_t i = 0; i < display_num; ++i) {
        free_item(items[i]);
    }

    endwin();
    return "";
}

void 
ProjectWindow::print_in_middle(WINDOW *win, int starty, int startx, int width, const char *string, chtype color)
{       int length, x, y;
        float temp;

        if(win == NULL)
                win = stdscr;
        getyx(win, y, x);
        if(startx != 0)
                x = startx;
        if(starty != 0)
                y = starty;
        if(width == 0)
                width = 80;

        length = strlen(string);
        temp = (width - length)/ 2;
        x = startx + (int)temp;
        wattron(win, color);
        mvwprintw(win, y, x, "%s", string);
        wattroff(win, color);
        refresh();
}

// string 
// ProjectWindow::menu(vector<string> print_array)
// {
//     int ch, mrows, mcols;
//     WINDOW *win,*subwin;
//     MENU *mymenu;
 
//     //初始化屏幕
//     initscr();
 
//     //不用等待回车键
//     cbreak();
 
//     //不回显
//     noecho();
 
//     //可以处理功能键 
//     keypad(stdscr,TRUE); 
 
//     //建立菜单项
//     vector<ITEM*> items;
//     items.push_back(new_item("1", "输入项目地址"));
//     for (std::size_t i = 0; i < print_array.size(); ++i) {
        
//     }
//   for(int i = 0; i < CITEM; i++){
 
//     items[i]=new_item(menus[i],menus[i]);//第二个参数为菜单项的描述
 
//   } 
 
//   //建立菜单
 
//   mymenu=new_menu(items);
 
//   set_menu_format(mymenu,CITEM,1);  //设置CITEM行1列的菜单
 
//   set_menu_mark(mymenu,">");//菜单选中的MARK
 
//   //获得菜单的行列数
 
//   scale_menu(mymenu,&mrows,&mcols);
 
//   //建立窗口和子窗口
 
//   win=newwin(mrows+2,mcols+2,3,30);
 
//   keypad(win,TRUE);
 
//   box(win,0,0);
 
//   subwin=derwin(win,0,0,1,1);
 
//   //设置菜单的窗口
 
//   set_menu_sub(mymenu,subwin);  
 
//   //在子窗口上放置菜单
 
//   post_menu(mymenu);
 
   
 
//   refresh();
 
//   wrefresh(win);
 
//   //获得输入，并移动选择到相应的菜单项 
 
//   while(toupper(ch=wgetch(win))!='\n'){
 
//      if(ch==KEY_DOWN)
 
//        menu_driver(mymenu,REQ_DOWN_ITEM);//移动菜单选择
 
//      else if(ch==KEY_RIGHT)
 
//        menu_driver(mymenu,REQ_RIGHT_ITEM);
 
//      else if (ch==KEY_UP)
 
//        menu_driver(mymenu,REQ_UP_ITEM); 
 
//      else if (ch==KEY_LEFT)
 
//        menu_driver(mymenu,REQ_LEFT_ITEM);
 
//   }
 
//   //输出当前项
 
//   mvprintw(LINES-2,0,"you select the item :%s\n",item_name(current_item(mymenu)));
 
//   refresh();
 
//   unpost_menu(mymenu);
 
//   getch();
 
//   //释放内存
 
//   free_menu(mymenu);
 
//   for(i=0;i<CITEM;i++) free_item(items[i]);
 
//   endwin();
// }