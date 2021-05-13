#include "project_window.h"

vector<string> chooses = {
                        "Choice 1",
                        "Choice 2",
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
                        "Exit"
                  };

int main(void)
{
    ProjectWindow pw;
    pw.display_menu(chooses, 0, chooses.size() - 1);
    return 0;
}