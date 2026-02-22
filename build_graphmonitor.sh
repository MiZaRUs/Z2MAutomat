#!/bin/bash

# Сборка Qt приложения.
APP=graphmon
#
QMake=qmake6
#
if [ -d ./Build ]; then
    rm -R Build/*
else
    mkdir Build
fi
#
cd ./Build/
$QMake ../src_graphmon
if [ -f "Makefile" ]; then
    make
else
    cd ..
    echo ""
    echo "ERROR: qmake!"
    echo ""
    exit
fi

if [[ -f "${APP}" && $(stat -c %s "${APP}") -gt 400 ]]; then
    echo ""
    echo "$(date "+%F %H:%M:%S")"
#    mv $APP ../
    echo "   ***"
else
    echo ""
    echo " * ERROR: Компиляция безуспешна! *"
    echo ""
fi
cd ..
