TEMPLATE	 = app
TARGET		 = graphmon

CONFIG += c++17

QT += widgets network

HEADERS		+= monitor.h WxGraph.h
SOURCES		+= main.cpp monitor.cpp WxGraph.cpp
