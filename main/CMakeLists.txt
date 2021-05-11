
project(project_manager)

set(EXECUTABLE_OUTPUT_PATH /home/ruanjian/workspace/project_pre/project_manager/output/debug/bin)

add_executable(Test_ncurses Test_ncurses.cc)

target_link_libraries(Test_ncurses -lproject_manager)
target_link_libraries(Test_ncurses -lgmock)
target_link_libraries(Test_ncurses -lgmock_main)
target_link_libraries(Test_ncurses -lgtest)
target_link_libraries(Test_ncurses -lgtest_main)
target_link_libraries(Test_ncurses -lpthread)
target_link_libraries(Test_ncurses -ldata_structure)
target_link_libraries(Test_ncurses -lsystem_util)
target_link_libraries(Test_ncurses -lprotocol)
target_link_libraries(Test_ncurses -lmessage)
target_link_libraries(Test_ncurses -lbuffer)
target_link_libraries(Test_ncurses -lncurses)

