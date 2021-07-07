#include "basic_head.h"
#include "project.h"
#include "cmake.h"
#include "exe_shell.h"

enum ProjectManagerOperate {
    PMO_Unknown = -1,
    PMO_Help = 0,
    PMO_Load,
    PMO_CreateProject,
    PMO_Build,
    PMO_Rebuild,
    PMO_ConfigureCFG,
    PMO_RunProgram,
    PMO_CleanAll,
    PMO_EnterExeDir,
    PMO_NewTerminter,
    PM0_PrintProjectName,
    PMO_AddRemoteGitAddr,
    PMO_PushToGithub,
    PMO_PullFromGithub,
    PMO_CreateTmpProject,
    PMO_PushToLocal,
    PMO_PullFromLocal,
};

void print_help(void);
int param_argv(int argc, char **argv);

int main(int argc, char **argv)
{
    Project proj;
    int ret = proj.init();
    if (ret < 0) {
        return 0;
    }
    ret = param_argv(argc, argv);
    switch (ret)
    {
        case PMO_Help:
        {
            print_help();
        } break;
        case PMO_Load:
        {
            std::cout << "PMO_Load" << std::endl;
            proj.load_project();
        } break;
        case PMO_CreateProject:
        {
            std::cout << "PMO_CreateProject" << std::endl;
            proj.create_project();
        } break;
        case PMO_Build:
        {
            CMake cmake(proj);
            cmake.create_top_level_cmakefile();
            cmake.build_project(true);
            std::cout << "PMO_Rebulid" << std::endl;
        } break;
        case PMO_Rebuild:
        {
            CMake cmake(proj);
            cmake.create_top_level_cmakefile();
            cmake.build_project(true);
            std::cout << "PMO_Rebulid" << std::endl;
        } break;
        case PMO_ConfigureCFG:
        {
            std::cout << "PMO_ConfigureCFG" << std::endl;
            proj.modify_config();
        } break;
        case PMO_RunProgram:
        {
            std::cout << "PMO_RunProgram" << std::endl;
        } break;
        case PMO_CleanAll:
        {
            std::cout << "PMO_CleanAll" << std::endl;
        } break;
        case PMO_EnterExeDir:
        {
            std::cout << "PMO_EnterExeDir" << std::endl;
        } break;
        case PMO_NewTerminter:
        {
            std::cout << "PMO_NewTerminter" << std::endl;
        } break;
        case PM0_PrintProjectName:
        {
            std::cout << "PM0_PrintProjectName" << std::endl;
        } break;
        case PMO_AddRemoteGitAddr:
        {
            std::cout << "PMO_AddRemoteGitAddr" << std::endl;
        } break;
        case PMO_PushToGithub:
        {
            std::cout << "PMO_PushToGithub" << std::endl;
        } break;
        case PMO_CreateTmpProject:
        {
            if (proj.is_load_any_project() != true) {
                LOG_GLOBAL_ERROR("Not load any project.");
                break;
            }
            chdir(proj.get_project_path().c_str());
            string result;
            exe_shell_cmd(result, "cp ../config/project_manager/tmp/main.cc ./main/");
            exe_shell_cmd(result, "cp ../config/project_manager/tmp/hello_world_sdk.h ./inc/");
            exe_shell_cmd(result, "cp ../config/project_manager/tmp/hello_world_sdk.cc ./src/");
            exe_shell_cmd(result, "cp ../config/project_manager/tmp/basic_header.h ./extern_inc/");
            exe_shell_cmd(result, "cp -r ../config/project_manager/tmp/hello ./");
            exe_shell_cmd(result, "cp -r ../config/project_manager/tmp/world ./");
        } break;
        default:
            print_help();
            break;
        }
    return 0;
}

void print_help(void)
{
    std::cout <<  "Usage: $0 [OPTIONS] [param1 param2 ...]." << std::endl;
    std::cout <<  "-h                    -- 打印帮助信息." << std::endl;
    std::cout <<  "-l                    -- 加载项目." << std::endl;
    std::cout <<  "-cp                   -- 创建新项目." << std::endl;
    std::cout <<  "-r                    -- 生成项目." << std::endl;
    std::cout <<  "-cr                   -- 重新生成项目." << std::endl;
    std::cout <<  "-cfg                  -- 项目配置文件" << std::endl;
    std::cout <<  "-run                  -- 运行程序." << std::endl;
    std::cout <<  "-c                    -- 清理." << std::endl;
    std::cout <<  "-e                    -- 进入可执行文件目录." << std::endl;
    std::cout <<  "-w                    -- 打开新的终端." << std::endl;
    std::cout <<  "-pp                   -- 打印当前项目名称." << std::endl;
    std::cout <<  "-rg                   -- 添加远程Github项目地址." << std::endl;
    std::cout <<  "-push                 -- 推送到GitHub上." << std::endl;
    std::cout <<  "-pull                 -- 从GitHub上拉取代码." << std::endl;
    std::cout <<  "-t                    -- 调试项目" << std::endl;
    std::cout <<  "" << std::endl;

    return ;
}

int param_argv(int argc, char **argv)
{
    if (argc != 2) {
        return PMO_Unknown;
    }

    vector<string> cmd = {"-h", "-l", "-cp","-r","-cr","-cfg","-run","-c","-e","-w","-pp","-rg","-push", "-pull","-t"};
    for (std::size_t i = 0; i < cmd.size(); ++i) {
        if (cmd[i] == argv[1]) {
            return static_cast<ProjectManagerOperate>(i);
        }
    }

    return PMO_Unknown;
}