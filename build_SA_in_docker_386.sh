#!/bin/bash

# Сборка докер-образа сервиса.
#
APP=z2m_automation
SRC=src_automat
####
#
if [ -d ./Build ]; then
    rm -R Build/*
else
    mkdir Build
fi
#
mkdir ./Build/srv
cd ./Build/srv
CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -a -installsuffix cgo -o $APP ../../$SRC
if [ -f "$APP" ]; then
    cd ../
    cp -r ../Docker ./
    echo "CMD [\"./${APP}\"]" >> ./Docker/Dockerfile
    tar -czvf ./Docker/srv.tar.gz srv
    cd ./Docker
#    docker rmi $APP
#    docker build --label=$APP -t $APP .
#    docker buildx build --platform linux/arm64 --label=$APP -t $APP .
    docker buildx build --platform linux/i386 --label=$APP -t $APP .
    cd ..
    docker save $APP > ${APP}.tar
    docker rmi $APP
else
    echo ""
    echo " * ERROR: Компиляция безуспешна! *"
    echo ""
fi
#docker images
########################

if [[ -f "${APP}.tar" && $(stat -c %s "${APP}.tar") -gt 400 ]]; then
    echo ""
    echo "$(date "+%F %H:%M:%S")  scp >>"
#    scp "${APP}.tar" buka.home:/opt/Z2MAutomat/
    echo "   ***"
else
    echo ""
    echo " * ERROR: Что-то пошло не так! *"
    echo ""
fi
