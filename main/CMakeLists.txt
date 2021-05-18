
project(project_manager)

set(EXECUTABLE_OUTPUT_PATH /home/ruanjian/workspace/project_pre/project_manager/output/debug/bin)

add_executable(Test_project_window Test_project_window.cc)

target_link_libraries(Test_project_window -lproject_manager)
target_link_libraries(Test_project_window -lgmock)
target_link_libraries(Test_project_window -lgmock_main)
target_link_libraries(Test_project_window -lgtest)
target_link_libraries(Test_project_window -lgtest_main)
target_link_libraries(Test_project_window -lpthread)
target_link_libraries(Test_project_window -ldata_structure)
target_link_libraries(Test_project_window -lsystem_util)
target_link_libraries(Test_project_window -lprotocol)
target_link_libraries(Test_project_window -lmessage)
target_link_libraries(Test_project_window -lbuffer)
target_link_libraries(Test_project_window -lncurses)
target_link_libraries(Test_project_window -lmenu)
target_link_libraries(Test_project_window -lform)

