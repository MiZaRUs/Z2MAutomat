/****************************************************************************
 *            Created    2026    by    oleg@shirokov.online                 *
 ****************************************************************************/

#include "monitor.h"
#include <QtNetwork>

//#include <QJsonObject>
//#include <QJsonDocument>
//#include <QJsonArray>
//#include <QJsonValue>
//#include <QJsonParseError>
//#include <QString>
//#include <quint64>

//---------------------------------------------------------------------------

Monitor::Monitor(AppConf* cfg, QWidget* pwgt/*= 0*/) : QWidget(pwgt){
    setCursor(Qt::PointingHandCursor);
    setMinimumSize(180,273);
    resize(400,600);
    setWindowTitle("Сделай выбор");

    url = cfg->url;
//    qDebug() << "DATA URL:" << url;

    pmenu = new QMenu(this);
    int pn = 0;
    for (auto& c : cfg->menu){
        pn++;
        QString strpn = QString::number(pn);
        pmenu->addSeparator();
        pmenu->addAction(c.graph)->setObjectName(strpn);
        config[strpn] = c;
//        qDebug() << "+M:" << pn << c.graph;	// WxMenu{graph, int spidtm, int scale, double factor, vector<GrData> data;}
    }
    pmenu->addSeparator();
    pmenu->addAction(tr( "&Oбновить"));
    pmenu->addSeparator();
    pmenu->addAction(tr("Вы&xод"));
    pmenu->addSeparator();
    connect(pmenu, SIGNAL(triggered(QAction*)),SLOT(slotMenuClicked(QAction*)));

    plout = new QVBoxLayout;
    plout->setContentsMargins(2, 2, 2, 2);
    plout->setSpacing(5);

    pgrd = new WxGraph();
    plout->addWidget(pgrd, 4);
    setLayout(plout);

    timer = new QTimer(this);
    timer->setInterval(1000);         //60000=1min// 20000=20sek
    connect(timer, SIGNAL(timeout()), this, SLOT(slotTimerRefresh()));
    timer->start();
}//End WxMain

//---------------------------------------------------------------------------

double Monitor::getData(int n, QString uid, QString sensor, qint64 tmin, qint64 tmax ){
    QJsonObject jsonObject;
    jsonObject["uid"] = uid;
    jsonObject["sensor"] = sensor;
    jsonObject["tmin"] = tmin;
    jsonObject["tmax"] = tmax;
    QByteArray postData = QJsonDocument(jsonObject).toJson(QJsonDocument::Compact);
//qDebug() << "Get DT:" << tmax-tmin << postData;

    QNetworkRequest request(url);
    request.setHeader(QNetworkRequest::ContentTypeHeader, "application/json");

    QNetworkAccessManager *manager = new QNetworkAccessManager();
    QNetworkReply *http = manager->post(request, postData);
    QEventLoop eventLoop;
    QObject::connect(http,SIGNAL(finished()),&eventLoop, SLOT(quit()));
    eventLoop.exec();

    QByteArray jstr = "{\"data\":"+ http->readAll() +"}";
//qDebug() << "JSON:" << jstr;
    QJsonParseError parseError;
    QJsonDocument doc = QJsonDocument::fromJson(jstr, &parseError);
    if (parseError.error != QJsonParseError::NoError) {
        qDebug() << "ERROR JSON parse:" << parseError.errorString() << parseError.offset;
        return -1;
    }

    double tmf = -1;
    double val = -1;
    if(tmax == 0) tmax = QDateTime::currentDateTime().toMSecsSinceEpoch();	// был запрос последних значений
    if (doc.isObject()){
        QJsonObject json = doc.object();
        QJsonArray jsonArray = json["data"].toArray();
        for(const QJsonValue &value : jsonArray){
            if (value.isObject()){
                QJsonObject obj = value.toObject();
                val = obj["val"].toDouble() * yfactor;
		tmf = (tmax - static_cast<quint64>(obj["tmu"].toDouble())) * xfactor;
//        qDebug() << "V:" << tmf << val;
                pgrd->trends[n].Points.push_back(QPointF(tmf, val));
            }
        }
    }
    pgrd->trends[n].Points.push_back(QPointF(0, val));
    return tmf;
}// End slot

//---------------------------------------------------------------------------

void Monitor::slotMenuClicked(QAction* pAction){
    QString strmn = pAction->text().remove("&");
    if(strmn == tr("Выxод"))slotActivExit();

//    qDebug() << "MenuClicked:" << strmn  << " : " << pAction->objectName();

    auto &c = config[pAction->objectName()];

    qDebug() << "+M:" << strmn << pAction->objectName() << c.graph << c.spidtm << c.scale << c.factor << c.data.size();

    if(strmn == "" || pAction->objectName() == "" || c.graph == "" || c.data.size() < 1 ) return;

    setWindowTitle(strmn);


//Количество данных зависит от видимой шкалы !!!
    int xscale = pgrd->getLenXScale()*600; //- размер шкалы в секундах
    xscale += xscale/4;		// плюс ещё четверть.

    curDTime = QDateTime::currentDateTime();	// Время "начала"
    auto tmax = curDTime.toMSecsSinceEpoch();
    auto tmin = curDTime.addSecs(-1 * xscale).toMSecsSinceEpoch();

//    int hour = 4;				// количество часов в запросе
//    auto tmin = curDTime.addSecs(-1 * (3600 * hour)).toMSecsSinceEpoch();
//qDebug() << "Get DT:" << tmin << tmax << tmax - tmin;

    QTime tm = curDTime.time();
//    pgrd->setPosition(tm.minute(), tm.second(), 59);	// шкала времени "минуты"
    pgrd->setPosition(tm.hour(), tm.minute(), 23);	// шкала времени "часы"
    xfactor = 0.00001 * (60.0 / float(20));	// множитель шкалы X  минуты.секунды;  60 минут / (xmm = 20)


    yfactor = c.factor;	// множитель шкалы Y - один к одному
    pgrd->setScale(c.scale);	// 0..50,  0..2

//    pgrd->Clear();			    		// Очистим данные трендов
    pgrd->trends.clear();				// Очистим тренды
    int it = 0;
    for(auto &d : c.data){
        qDebug() << " +D:" << d.uid << d.sensor << d.title << d.color;  // GrData{uid, sensor, title, QColor color }
        pgrd->trends.push_back(Trend(d.color));
        getData(it, d.uid, d.sensor, tmin, tmax );	// double tmu		Кухня
        it++;
    }
    pgrd->update();
    return;
}// End slotMenuClicked

//---------------------------------------------------------------------------

void Monitor::slotTimerRefresh(){
    curDTime = QDateTime::currentDateTime();       // Время
    QTime tm = curDTime.time();

    int hour = tm.hour();         // Get the hour (0-23)
    int minute = tm.minute();     // Get the minute (0-59)
    int second = tm.second();     // Get the second (0-59)

    if((second % 10) == 0){	// НАДО сделать выбор секунды минуты  (10сек, 1мин, 10мин)

qDebug() << "WxMain::slotTimerRefresh():" << hour << ":" << minute << ":" << second << " Scl:" << pgrd->getLenXScale()*600;

        pgrd->setPosition(hour, minute, 23);	// шкала времени
//        pgrd->setPosition(minute, second, 59);	// шкала времени

        pgrd->update();
    }
}// End slot

//void WxMain::refreshDate(){
//qDebug() << tr("WxMain::refreshDate()");
//    return;
//}// End slot

//---------------------------------------------------------------------------

void Monitor::keyPressEvent(QKeyEvent *event){
//qDebug() << tr("Kl >>") << event->key();
    switch (event->key()) {
    case 16777264:	// F1
        slotActivHelp();
        break;

    case 16777268:	// F5
        slotTimerRefresh();
        break;

    case 16777272:	// F9
    case 16777216:	// esc
    case 16777273:	// F10
        slotActivExit();
        break;

    default:
        QWidget::keyPressEvent(event);
    }
}

//---------------------------------------------------------------------------

void Monitor::slotActivHelp(){
    QTextEdit *txt = new QTextEdit;
    txt->setReadOnly(true);
    txt->setHtml( tr("<HTML>"
        "<BODY>"
        "<H2><CENTER> SMonitor </CENTER></H2>"
        "<P ALIGN=\"left\">"
            "Просмотр графиков"
            "F5 обновить"
            "esc F9 F10 завершить"
            "<BR>"
            "<BR>"
            "<BR>"
        "</P>"
        "<H3><CENTER> Версия 0.4 </CENTER></H3>"
        "<H4><CENTER> Февраль 2026 </CENTER></H4>"
        "<H4><CENTER> oleg@shirokov.online </CENTER></H4>"
        "<BR>"
        "</BODY>"
        "</HTML>"
    ));
    txt->resize(300, 200);
    txt->show();
    return;
}// End slot

//---------------------------------------------------------------------------

void Monitor::slotActivExit(){
    close();
    return;
}// End slot

//---------------------------------------------------------------------------
