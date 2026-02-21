/****************************************************************************
 *            Created    2026    by    oleg@shirokov.online                 *
 ****************************************************************************/

#include <QApplication>
#include "monitor.h"

#include <QFile>
#include <QByteArray>
#include <QJsonObject>
#include <QJsonDocument>
#include <QJsonArray>
#include <QJsonValue>
#include <QJsonParseError>
//#include <QDebug>
//---------------------------------------------------------------------------

AppConf getConfig(QString fn);

//---------------------------------------------------------------------------

int main( int argc, char *argv[] ){
    QApplication app( argc, argv );

    AppConf config = getConfig("./conf/monitor.json");
    if(config.url == "") return -1;

    Monitor gwx(&config, 0);
//    gwx.setStyleSheet("background-color: black;");

    gwx.show();
    return app.exec();
}

//---------------------------------------------------------------------------

AppConf getConfig(QString fn){	// Читаем конфиг
    AppConf ac;
    QFile f(fn);
    if (!f.open(QIODevice::ReadOnly | QIODevice::Text)){
        qDebug() << "ERROR getConfig: failed to open";
        return ac;
    }
    QByteArray jstr = f.readAll();
//    qDebug() << "read" << jstr.size();
//    qDebug() << "file size is" << f.size();
//qDebug() << "JSON:" << jstr;

    QJsonParseError parseError;
    QJsonDocument doc = QJsonDocument::fromJson(jstr, &parseError);
    if (parseError.error != QJsonParseError::NoError) {
        qDebug() << "ERROR JSON parse:" << parseError.errorString() << parseError.offset;
        return ac;
    }

    if (doc.isObject()){
        QJsonObject json = doc.object();
        ac.url = json["url"].toString();
        if(ac.url == ""){ return ac; }
        for(const QJsonValue &menu : json["menu"].toArray()){
            if (menu.isObject()){
                QJsonObject obj = menu.toObject();
                QString  graph = obj["graph"].toString();
                if(graph != "") {
                    std::vector<GrData> data;
                    for(const QJsonValue &val : obj["data"].toArray()){
                        if (val.isObject()){
                            QJsonObject obj = val.toObject();
                            QString  uid = obj["uid"].toString();
                            QString  sensor = obj["sensor"].toString();
                            QString  title = obj["title"].toString();

                            int i = 0;
                            int col[3] = {0,0,0};	// R G B
                            for(const QJsonValue & color : obj["color"].toArray()){
                                col[i] = (int)color.toDouble();
                                if((i++) > 2) break;
                            }
                            if(uid != "" && sensor != "" && title != ""){
                                data.push_back({ uid, sensor, title, QColor(col[0], col[1], col[2]) });
                            }
                        }
                    }
                    ac.menu.push_back({graph, obj["spidtm"].toInt(), obj["scale"].toInt(), obj["factor"].toDouble(), data });
                }
            }
        }
    }
    return ac;
}

//---------------------------------------------------------------------------
