
project(project_manager)

set(EXECUTABLE_OUTPUT_PATH /home/ruanjian/workspace/project_pre/project_manager/output/debug/bin)

add_executable(main main.cc)

target_link_libraries(main -lproject_manager)
target_link_libraries(main -lgmock)
target_link_libraries(main -lgmock_main)
target_link_libraries(main -lgtest)
target_link_libraries(main -lgtest_main)
target_link_libraries(main -lpthread)
target_link_libraries(main -ldata_structure)
target_link_libraries(main -lsystem_util)
target_link_libraries(main -lprotocol)
target_link_libraries(main -lmessage)
target_link_libraries(main -lbuffer)

