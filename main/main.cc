#include "basic_head.h"
#include "project.h"

enum ProjectManagerOperate {
    PMO_Unknown = -1,
    PMO_Help = 0,
    PMO_Load,
    PMO_CreateProject,
    PMO_BuildProjectWithDebug,
    PMO_BuildProjectWitchRelease,
    PMO_Rebulid,
    PMO_ConfigureCFG,
    PMO_RunProgram,
    PMO_CleanAll,
    PMO_EnterExeDir,
    PMO_NewTerminter,
    PM0_PrintProjectName,
    PMO_AddRemoteGitAddr,
    PMO_PushToGithub
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
        case PMO_BuildProjectWithDebug:
        {
            std::cout << "PMO_BuildProjectWithDebug" << std::endl;
        } break;
        case PMO_BuildProjectWitchRelease:
        {
            std::cout << "PMO_BuildProjectWitchRelease" << std::endl;
        } break;
        case PMO_Rebulid:
        {
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
    std::cout <<  "-r                    -- 生成项目(debug)." << std::endl;
    std::cout <<  "-rr                   -- 生成项目(release)." << std::endl;
    std::cout <<  "-cr                   -- 重新生成项目." << std::endl;
    std::cout <<  "-cfg                  -- 项目配置文件" << std::endl;
    std::cout <<  "-run                  -- 运行程序." << std::endl;
    std::cout <<  "-c                    -- 清理." << std::endl;
    std::cout <<  "-e                    -- 进入可执行文件目录." << std::endl;
    std::cout <<  "-w                    -- 打开新的终端." << std::endl;
    std::cout <<  "-pp                   -- 打印当前项目名称." << std::endl;
    std::cout <<  "-rg                   -- 添加远程Github项目地址." << std::endl;
    std::cout <<  "-push                 -- 推送到GitHub上." << std::endl;
    std::cout <<  "" << std::endl;

    return ;
}

int param_argv(int argc, char **argv)
{
    if (argc != 2) {
        return PMO_Unknown;
    }

    vector<string> cmd = {"-h", "-l", "-cp","-r","-rr","-cr","-cfg","-run","-c","-e","-w","-pp","-rg","-push"};
    for (std::size_t i = 0; i < cmd.size(); ++i) {
        if (cmd[i] == argv[1]) {
            return static_cast<ProjectManagerOperate>(i);
        }
    }

    return PMO_Unknown;
}