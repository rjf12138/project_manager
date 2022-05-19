function generate_project()
{
    if [ -d $1 ]
    then
        echo "$1 is exists"
        exit 1
    fi

    PROJECT_NAME=$2
    PROJECT_PATH=$1

    # 创建项目目录和配置文件
    mkdir $PROJECT_PATH/
    mkdir $PROJECT_PATH/config
    mkdir $PROJECT_PATH/doc
    mkdir $PROJECT_PATH/inc
    mkdir $PROJECT_PATH/lib
    mkdir $PROJECT_PATH/lib/debug
    mkdir $PROJECT_PATH/lib/release
    mkdir $PROJECT_PATH/main
    mkdir $PROJECT_PATH/src
    mkdir $PROJECT_PATH/output
    mkdir $PROJECT_PATH/output/debug
    mkdir $PROJECT_PATH/output/debug/bin
    mkdir $PROJECT_PATH/output/debug/lib
    mkdir $PROJECT_PATH/output/release
    mkdir $PROJECT_PATH/output/release/bin
    mkdir $PROJECT_PATH/output/release/lib
    mkdir $PROJECT_PATH/.proj_config
    mkdir $PROJECT_PATH/.vscode

    # 创建项目配置文件和cmake编译文件
    # 只支持在当前所在目录下创建文件

    # 添加一些基本的头文见和测试库
    cp -rf $EXTERN_RESOURSE_PATH/gtest/include/*            $PROJECT_PATH/inc/
    cp -rf $EXTERN_RESOURSE_PATH/gtest/lib/*                $PROJECT_PATH/lib/debug
    cp -rf $EXTERN_RESOURSE_PATH/basic_head.h               $PROJECT_PATH/inc/
    cp -rf $EXTERN_RESOURSE_PATH/project_config.json        $PROJECT_PATH/.proj_config/
    cp -rf $EXTERN_RESOURSE_PATH/exbin/parse_json           $PROJECT_PATH/.proj_config/
    chmod u+x $PROJECT_PATH/.proj_config/parse_json

    # 生成项目uuid
    project_uuid=`uuidgen`
    echo $PROJECT_PATH" "$PROJECT_NAME
    
    # 加载项目信息到配置中
    echo "`cat $TMP_PROJECT_INFO | grep install_path`" > $TMP_PROJECT_INFO
    echo "project_config=$PROJECT_PATH/.proj_config/project_config.json" >> $TMP_PROJECT_INFO
    echo "project_name=$PROJECT_NAME" >> $TMP_PROJECT_INFO
    echo "project_uuid=$project_uuid" >> $TMP_PROJECT_INFO
    echo "project_path=$PROJECT_PATH" >> $TMP_PROJECT_INFO

    # 初始化配置文件
    set_obj_val 项目名称 "$PROJECT_PATH"
    set_obj_val 项目名称 "$PROJECT_NAME"
    set_obj_val 项目UUID "$project_uuid"

    # 初始化git
    cd $PROJECT_PATH
    git init
    git add -A .
	git commit -m "first commit"
}