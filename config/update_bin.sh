#!/bin/bash

if [ ! -f ../main ]
then
    echo "Can't find exec file[main]"
    exit 127
fi

cd project_manager_bin
git pull
cp ../../main ./program/project

git add -A .
git commit -m "`date`"
git push -u origin master

exit 0