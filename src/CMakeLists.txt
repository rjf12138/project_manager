project(project_manager)

set(LIBRARY_OUTPUT_PATH /home/ruanjian/workspace/project_pre/project_manager/output/debug/lib)

aux_source_directory(. DIR_LIB_SRCS)

add_library(project_manager ${DIR_LIB_SRCS})

target_link_libraries(project_manager -lpthread)
target_link_libraries(project_manager -ldata_structure)
target_link_libraries(project_manager -lsystem_util)
target_link_libraries(project_manager -lprotocol)
target_link_libraries(project_manager -lmessage)
target_link_libraries(project_manager -lbuffer)
target_link_libraries(project_manager -lncurses)
target_link_libraries(project_manager -lmenu)
target_link_libraries(project_manager -lform)

