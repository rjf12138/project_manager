#!/bin/bash

PROJECT_INSTALL_PATH=/usr/local/bin
if [ ! -d $PROJECT_INSTALL_PATH ]
then
    echo "Can't find $PROJECT_INSTALL_PATH".
    exit 127
fi

if [ -f $PROJECT_INSTALL_PATH/project_manager ]
then
    sudo rm -rf $PROJECT_INSTALL_PATH/project_manager/*
fi

