#!/bin/bash

# 默认安装位置
PROJECT_INSTALL_PATH=/usr/local/bin
if [ ! -d $PROJECT_INSTALL_PATH ]
then
    echo "Can't find $PROJECT_INSTALL_PATH"
    exit 127
fi

git clone git@github.com:rjf12138/project_manager_bin.git

if [ -d ./project_manager_bin ]
then
    sudo cp -rf ./project_manager_bin/ $PROJECT_INSTALL_PATH/
    rm -rf ./project_manager_bin
    cd $PROJECT_INSTALL_PATH/project_manager_bin
    # 记录项目安装目录,设置可执行目录
    project_path=`pwd`
    echo $project_path > $HOME/.project_manager.ini
    result=`cat $HOME/.bashrc | grep $project_path/program`
    echo $result
    if [ -z "$result" ]
    then
        echo "$PATH"
        echo "$project_path/program"
        echo "export PATH=$PATH:$project_path/program" >> $HOME/.bashrc
    fi
    # 将更新文件进程设为可执行
    sudo chmod u+x ./program/update_project_manager
    sudo chmod u+x ./program/project
    echo "Install successed."
else
    echo "Can't download project_manager_bin."
fi

exit 0