#include "project_window.h"


ProjectWindow::ProjectWindow(void)
{
    
}

ProjectWindow::~ProjectWindow(void)
{
    
}

string 
ProjectWindow::display_menu(vector<string> proj_name, vector<string> proj_path)
{
    initscr();  // 以 curses 模式初始化终端
    start_color();
    cbreak();   // 当缓存中有数据可读时直接返回而不是等到换行符或是缓冲满了才返回
    noecho();   // 输入字符不回显
    keypad(stdscr, TRUE); // 支持读取功能键像 F1, F2, arrow keys etc
    init_pair(1, COLOR_RED, COLOR_BLACK);
    init_pair(2, COLOR_CYAN, COLOR_BLACK);

    vector<string> display_paths;
    for (std::size_t i = 0; i < proj_path.size(); ++i) {
        int32_t path_start = static_cast<int32_t>(proj_path[i].length()) - 48;
        if (path_start > 0) { // 路径过长只显示后面一部分
            std::string path = ".../";
            path.append(proj_path[i].c_str() + path_start);
            display_paths.push_back(path);
        } else {
            display_paths.push_back(proj_path[i]);
        }
    }

    std::size_t display_num = display_paths.size();
    ITEM** items = new ITEM*[display_num + 1];
    items[0] = new_item(ProjWin_InputPath, "");
    for (std::size_t i = 0; i < display_num; ++i) {
        items[i + 1] = new_item(proj_name[i].c_str(), display_paths[i].c_str());
    }

    MENU *project_menu = new_menu(items);
    WINDOW *menu_window = newwin(23, 79, 0, 0);
    keypad(menu_window, TRUE);
    
    /* Set main window and sub window */
    set_menu_win(project_menu, menu_window);
    set_menu_sub(project_menu, derwin(menu_window, 20, 74, 3, 1));
    set_menu_format(project_menu, 15, 1);

    /* Set menu mark to the string " * " */
    set_menu_mark(project_menu, " > ");

    /* Print a border around the main window and print a title */
    box(menu_window, 0, 0);
    this->print_in_middle(menu_window, 1, 0, 79, "Open Project", COLOR_PAIR(1));
    mvwaddch(menu_window, 2, 0, ACS_LTEE);
    mvwhline(menu_window, 2, 1, ACS_HLINE, 77);
    mvwaddch(menu_window, 2, 78, ACS_RTEE);

    post_menu(project_menu);
    wrefresh(menu_window);

    string curr_text;
    while(true) // 回车键
    {   
        int ch = wgetch(menu_window);
        if (ch == Keyboard_Enter) { // 回车键退出
            if (project_menu->curitem->description.length == 0) {
                curr_text = ProjWin_InputPath;
            } else {
                curr_text = proj_path[project_menu->curitem->index]; // 返回项目路径
            }
            break;
        } else if (ch == Keyboard_Esc) {
            curr_text = "";
            break;
        }

        switch(ch)
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

    return curr_text;
}

string 
ProjectWindow::get_input(string title)
{
    FIELD *field[5];
    FORM  *my_form;
    int ch;
    
    /* Initialize curses */
    initscr();
    cbreak();
    noecho();
    keypad(stdscr, TRUE);

    /* Initialize the fields */
    field[0] = new_field(1, 70, 8, 7, 0, 0);
    field[1] = new_field(1, 70, 10, 7, 0, 0);
    field[2] = new_field(1, 6, 12, 24, 0, 0);
    field[3] = new_field(1, 10, 12, 40, 0, 0);
    field[4] = nullptr;

    /* Set field options */
    field_opts_off(field[0], O_ACTIVE); /* This field is a static label */


    /* Create the form and post it */
    my_form = new_form(field);
    post_form(my_form);
    refresh();
    
    set_field_just(field[0], JUSTIFY_CENTER); /* Center Justification */
    set_field_buffer(field[0], 0, title.c_str()); 

    set_field_back(field[1], A_UNDERLINE);
    field_opts_off(field[1], O_AUTOSKIP); /* Don't go to next field when this Field is filled up */
    //field_opts_off(field[1], O_STATIC);

    set_field_just(field[2], JUSTIFY_CENTER); /* Center Justification */
    set_field_buffer(field[2], 0, "[ OK ]"); 
    field_opts_off(field[2], O_AUTOSKIP);
    field_opts_off(field[2], O_EDIT);

    set_field_just(field[3], JUSTIFY_CENTER); /* Center Justification */
    set_field_buffer(field[3], 0, "[ Cancel ]"); 
    field_opts_off(field[3], O_AUTOSKIP);
    field_opts_off(field[2], O_EDIT);
    
    mvprintw(8, 0, "Title: ");
    mvprintw(10, 0, "Input: ");
    refresh();

    /* Loop through to get user requests */
    string input_text;
    int input_char_count = 0;
    int current_cursor_pos = 0;
    while(true)
    {
        int ch = getch();
        if (ch == Keyboard_Enter) { // 回车键退出

            form_driver(my_form, REQ_VALIDATION);
            input_text = field_buffer(field[1], 0);
            break;
        } else if (ch == Keyboard_Esc) {
            input_text = "";
            break;
        }
        // 打印输入字符

        switch(ch)
        {
            case KEY_LEFT: {
                /* 移到上一个字符 */
                if (current_cursor_pos > 0) {
                    --current_cursor_pos;
                    form_driver(my_form, REQ_PREV_CHAR);
                }
            } break;
            case KEY_RIGHT: {
                /* 移到下一个字符 */
                if (current_cursor_pos < input_char_count) {
                    ++current_cursor_pos;
                    form_driver(my_form, REQ_NEXT_CHAR);
                }
            } break;
            case KEY_BACKSPACE: {
                if (input_char_count > 0) {
                    form_driver(my_form, REQ_DEL_PREV);
                    --input_char_count;
                }
            } break;
            case Keyboard_Tab: {
                form_driver(my_form, REQ_NEXT_FIELD);
                form_driver(my_form, REQ_END_LINE);
            } break;
            default: {
                if (input_char_count < Form_MaxInput) {
                    form_driver(my_form, ch);
                    ++input_char_count;
                    ++current_cursor_pos;
                }
            }
        }
        
    }
    
    /* Un post form and free the memory */
    unpost_form(my_form);
    free_form(my_form);
    free_field(field[0]);
    free_field(field[1]);

    endwin();
    return input_text;
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