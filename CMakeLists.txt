cmake_minimum_required(VERSION 3.10)

# 设置项目名称
project(project_manager)

set(CMAKE_DEBUG_POSTFIX d)

set(CXX_FLAGS
    -g
    # -DVALGRIND
    -DCHECK_PTHREAD_RETURN_VALUE
    -D_FILE_OFFSET_BITS=64
    -march=native
    # -MMD
    -rdynamic
    )
string(REPLACE ";" " " CMAKE_CXX_FLAGS "${CXX_FLAGS}")

# 设置生成文件输出路径
set(EXECUTABLE_OUTPUT_PATH, ./output/debug/bin)
add_executable(project_manager ./main/main.cc)

target_include_directories(project_manager PUBLIC ./inc/)
target_include_directories(project_manager PRIVATE ./extern_inc/)

target_link_directories(project_manager PRIVATE ./lib/debug/)

include(src/CMakeLists.txt)

