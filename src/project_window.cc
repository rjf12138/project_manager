#include "project_window.h"


ProjectWindow::ProjectWindow(void)
{
    initscr();  // 以 curses 模式初始化终端
    cbreak();   // 当缓存中有数据可读时直接返回而不是等到换行符或是缓冲满了才返回
    noecho();   // 输入字符不回显
    keypad(stdscr, TRUE); // 支持读取功能键像 F1, F2, arrow keys etc
}

ProjectWindow::~ProjectWindow(void)
{
    endwin();
}

string 
ProjectWindow::menu_manager(vector<string> print_array, int menu_display_num)
{
    std::size_t menu_page = print_array.size() / menu_display_num;
    menu_page = (print_array.size() % menu_display_num == 0 ? menu_page : menu_page + 1);
    

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

    std::size_t display_num = tail - start + 5;
    std::size_t index = 1;
    ITEM** items = new ITEM*[display_num];
    items[index++] = new_item(to_string(index).c_str(), "输入打开项目的地址");
    for (std::size_t i = start; i <= tail; ++i) {
        items[index++] = new_item(to_string(index).c_str(), print_array[i].c_str());
    }
    items[index++] = new_item(to_string(index).c_str(), "上一页");
    items[index] = new_item(to_string(index).c_str(), "下一页");

    MENU *project_menu = new_menu((ITEM **)items);
    mvprintw(LINES - 1, 0, "按 q 退出");
    post_menu(project_menu);
    refresh();

    char ch;
    while((ch = getch()) != KEY_F(1))
    {   switch(ch)
        {   
            case KEY_DOWN: {
                menu_driver(project_menu, REQ_DOWN_ITEM);
            } break;
            case KEY_UP: {
                menu_driver(project_menu, REQ_UP_ITEM);
            } break;
        }
    }       

    free_menu(project_menu);
    for (std::size_t i = 0; i < index; ++i) {
        free_item(items[i]);
    }
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