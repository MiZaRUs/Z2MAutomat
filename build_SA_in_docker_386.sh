#!/bin/bash

# Сборка докер-образа сервиса.
#
APP=z2m_automation
####
#
if [ -d ./Build ]; then
    rm -R Build/*
else
    mkdir Build
fi
#
cp -r ./Docker ./Build/
echo "CMD [\"./${APP}\"]" >> ./Build/Docker/Dockerfile
cp -r ./src_automat/*.go ./Build/
cd ./Build/
#GO111MODULE=off CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $APP
GO111MODULE=off CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -a -installsuffix cgo -o $APP
if [ -f "$APP" ]; then
    mkdir ./Docker/srv
    mv $APP ./Docker/srv/
    cd ./Docker
    tar -czvf srv.tar.gz srv
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
