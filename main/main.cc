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
    PMO_PushToLocal,
    PMO_PullFromLocal,
    PMO_CreateTmpProject,
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
            //std::cout << "PMO_Load" << std::endl;
            proj.load_project();
        } break;
        case PMO_CreateProject:
        {
            //std::cout << "PMO_CreateProject" << std::endl;
            proj.create_project();
            chdir(proj.get_project_path().c_str());
            proj.pull_file();
            system("git init");
            system("git add -A .");
	        system("git commit -m \"first commit\"");
        } break;
        case PMO_Build:
        {
            if (proj.is_load_any_project() == false) {
                LOG_GLOBAL_ERROR("Not load any project");
                return -1;
            }
            CMake cmake(proj);
            cmake.create_top_level_cmakefile();
            cmake.build_project(false);
            proj.generate_vscode_config();
            //std::cout << "PMO_Rebulid" << std::endl;
        } break;
        case PMO_Rebuild:
        {
            if (proj.is_load_any_project() == false) {
                LOG_GLOBAL_ERROR("Not load any project");
                return -1;
            }
            CMake cmake(proj);
            cmake.create_top_level_cmakefile();
            cmake.build_project(true);
            proj.generate_vscode_config();
            //std::cout << "PMO_Rebulid" << std::endl;
        } break;
        case PMO_ConfigureCFG:
        {
            if (proj.is_load_any_project() == false) {
                LOG_GLOBAL_ERROR("Not load any project");
                return -1;
            }
            //std::cout << "PMO_ConfigureCFG" << std::endl;
            proj.modify_config();
        } break;
        case PMO_RunProgram:
        {
            if (proj.is_load_any_project() == false) {
                LOG_GLOBAL_ERROR("Not load any project");
                return -1;
            }
            chdir(proj.get_project_bin_path().c_str());
            JsonString prog_arg = (*proj.get_project_config())["ProgramRunArgs"];
            string bin_path = proj.get_project_bin_path() + "/" + proj.get_project_name() + " " + prog_arg.value();
            cout << bin_path << endl;
            system(bin_path.c_str());
            //std::cout << "PMO_RunProgram" << std::endl;
        } break;
        case PMO_CleanAll:
        {
            if (proj.is_load_any_project() == false) {
                LOG_GLOBAL_ERROR("Not load any project");
                return -1;
            }
            CMake cmake(proj);
            cmake.clean_project();
            //std::cout << "PMO_CleanAll" << std::endl;
        } break;
        case PMO_EnterExeDir:
        {
            if (proj.is_load_any_project() == false) {
                LOG_GLOBAL_ERROR("Not load any project");
                return -1;
            }
            string result;
            exe_shell_cmd(result, "cd %s;gnome-terminal --tab", proj.get_project_bin_path().c_str());
            //std::cout << "PMO_EnterExeDir" << std::endl;
        } break;
        case PMO_NewTerminter:
        {
            system("gnome-terminal");
            //std::cout << "PMO_NewTerminter" << std::endl;
        } break;
        case PM0_PrintProjectName:
        {
            if (proj.is_load_any_project() == false) {
                LOG_GLOBAL_ERROR("Not load any project");
                return -1;
            }
            std::cout << "Project Name: " << proj.get_project_name() << std::endl;
            JsonString jsuuid = (*proj.get_project_config())["UUID"];
            std::cout << "Project UUID: " << jsuuid.value() << std::endl;
            //std::cout << "PM0_PrintProjectName" << std::endl;
        } break;
        case PMO_AddRemoteGitAddr:
        {
            if (proj.is_load_any_project() == false) {
                LOG_GLOBAL_ERROR("Not load any project");
                return -1;
            }
            string github_addr;
            ProjectWindow window;
            int ret = window.get_input(github_addr, "Input remote github address\n(git@github.com:XXXX/XXXXX.git)");
            if (ret < 0) {
                LOG_GLOBAL_DEBUG("Get github remote addr failed.");
                return 0;
            }
            string cmd = "git remote add origin ";
            cmd += github_addr;
            system(cmd.c_str());
            //std::cout << "PMO_AddRemoteGitAddr" << std::endl;
        } break;
        case PMO_PushToGithub:
        {
            if (proj.is_load_any_project() == false) {
                LOG_GLOBAL_ERROR("Not load any project");
                return -1;
            }
            system("git add -A .");
            system("git commit -m \"`date`\"");
            system("git push -u origin master");
            //std::cout << "PMO_PushToGithub" << std::endl;
        } break;
        case PMO_PullFromGithub:
        {
            if (proj.is_load_any_project() == false) {
                LOG_GLOBAL_ERROR("Not load any project");
                return -1;
            }
            system("git pull");
            //std::cout << "PMO_PullFromGithub" <<std::endl;
        } break;
        case PMO_PullFromLocal:
        {
            if (proj.is_load_any_project() == false) {
                LOG_GLOBAL_ERROR("Not load any project");
                return -1;
            }
            proj.pull_file();
            //std::cout << "PMO_PullFromLocal" <<std::endl;
        } break;
        case PMO_PushToLocal:
        {
            if (proj.is_load_any_project() == false) {
                LOG_GLOBAL_ERROR("Not load any project");
                return -1;
            }
            CMake cmake(proj);
            cmake.create_install_env();
            proj.push_file();
            //std::cout << "PMO_PushToLocal" <<std::endl;
        } break;
        case PMO_CreateTmpProject:
        {
            if (proj.is_load_any_project() != true) {
                LOG_GLOBAL_ERROR("Not load any project.");
                break;
            }
            chdir(proj.get_project_path().c_str());
            string result;
            exe_shell_cmd(result, "cp %s/tmp/main.cc ./main/", proj.get_project_install_path().c_str());
            exe_shell_cmd(result, "cp %s/tmp/hello_world_sdk.h ./inc/", proj.get_project_install_path().c_str());
            exe_shell_cmd(result, "cp %s/tmp/hello_world_sdk.cc ./src/", proj.get_project_install_path().c_str());
            exe_shell_cmd(result, "cp %s/tmp/basic_header.h ./extern_inc/", proj.get_project_install_path().c_str());
            exe_shell_cmd(result, "cp -r %s/tmp/hello ./", proj.get_project_install_path().c_str());
            exe_shell_cmd(result, "cp -r %s/tmp/world ./", proj.get_project_install_path().c_str());

            // 测试代码
            (*proj.get_project_config())["MainFileName"] = "main.cc";
            (*proj.get_project_config())["SourceFileDirectoryListing"].add("hello");
            (*proj.get_project_config())["SourceFileDirectoryListing"].add("world");
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
    std::cout <<  "-push_git             -- 推送代码到GitHub上." << std::endl;
    std::cout <<  "-pull_git             -- 从GitHub上拉取代码." << std::endl;
    std::cout <<  "-push_local           -- 推送生成文件到本地上." << std::endl;
    std::cout <<  "-pull_local           -- 从本地上拉取关联项目." << std::endl;
    //std::cout <<  "-t                    -- 调试项目" << std::endl;
    std::cout <<  "" << std::endl;

    return ;
}

int param_argv(int argc, char **argv)
{
    if (argc != 2) {
        return PMO_Unknown;
    }

    vector<string> cmd = {"-h", "-l", "-cp","-r","-cr","-cfg","-run","-c","-e","-w","-pp","-rg","-push_git", "-pull_git", "-push_local", "-pull_local","-t",};
    for (std::size_t i = 0; i < cmd.size(); ++i) {
        if (cmd[i] == argv[1]) {
            return static_cast<ProjectManagerOperate>(i);
        }
    }

    return PMO_Unknown;
}