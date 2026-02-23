/****************************************************************************
 *            Created    2026    by    oleg@shirokov.online                 *
 ****************************************************************************/

#include "monitor.h"

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
    setWindowTitle("F1-подсказка. F9-меню.");

    ptrCurMenu = nullptr;

    url = cfg->url;
//    qDebug() << "DATA URL:" << url;
    network = new QNetworkAccessManager();

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
    timer->setInterval(10000);         //60000=1min// 20000=20sek
    connect(timer, SIGNAL(timeout()), this, SLOT(slotTimerRefresh()));
    timer->start();
}//End Monitor

//---------------------------------------------------------------------------

double Monitor::getData(int n, QString uid, QString sensor, qint64 tmin, qint64 tmax, bool aprx ){
    QJsonObject jsonObject;
    jsonObject["uid"] = uid;
    jsonObject["sensor"] = sensor;
    jsonObject["tmin"] = tmin;
    jsonObject["tmax"] = tmax;
    QByteArray postData = QJsonDocument(jsonObject).toJson(QJsonDocument::Compact);
//qDebug() << "Get DT:" << tmax-tmin << postData;

    QNetworkRequest request(url);
    request.setHeader(QNetworkRequest::ContentTypeHeader, "application/json");

    QNetworkReply *http = network->post(request, postData);
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
    http->close();

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
                if(!aprx && pgrd->trends[n].Points.size() > 0) {
                    double vx = pgrd->trends[n].Points.back().ry();
                    pgrd->trends[n].Points.push_back(QPointF(tmf, vx));	// сдинем прошлое значение
                }

                pgrd->trends[n].Points.push_back(QPointF(tmf, val));
            }
        }
    }
    pgrd->trends[n].Points.push_back(QPointF(0, val));
    return tmf;
}// End slot

//---------------------------------------------------------------------------

void Monitor::refreshData(){
//qDebug() << tr("Monitor::refreshData()");
    if(ptrCurMenu == nullptr) return;

//Количество данных зависит от видимой шкалы !!!
    int xscale = pgrd->getLenXScale()*600; //- размер шкалы в секундах
    xscale += xscale/4;		// плюс ещё четверть.

    curDTime = QDateTime::currentDateTime();	// Время "начала"
    auto tmax = curDTime.toMSecsSinceEpoch();
    auto tmin = curDTime.addSecs(-1 * xscale).toMSecsSinceEpoch();


    QTime tm = curDTime.time();
//    pgrd->setPosition(tm.minute(), tm.second(), 59);	// шкала времени "минуты"
    pgrd->setPosition(tm.hour(), tm.minute(), 23);	// шкала времени "часы"
    xfactor = 0.00001 * (60.0 / float(20));	// множитель шкалы X  минуты.секунды;  60 минут / (xmm = 20)


    yfactor = ptrCurMenu->factor;	// множитель шкалы Y - один к одному
    pgrd->setScale(ptrCurMenu->scale);	// 0..50,  0..2

    pgrd->trends.clear();				// Очистим тренды
    int it = 0;
    for(auto &d : ptrCurMenu->data){
        qDebug() << " +D:" << d.uid << d.sensor << d.title << d.color;  // GrData{uid, sensor, title, QColor color }
        pgrd->trends.push_back(Trend(d.color));
        getData(it, d.uid, d.sensor, tmin, tmax, ptrCurMenu->approxim );	// double tmu
        it++;
    }
    return;
}// End slot

//---------------------------------------------------------------------------

void Monitor::slotMenuClicked(QAction* pAction){
    QString strmn = pAction->text().remove("&");
    if(strmn == tr("Выxод"))slotActivExit();
//    qDebug() << "MenuClicked:" << strmn  << " : " << pAction->objectName();

    ptrCurMenu = &config[pAction->objectName()];

//    qDebug() << "+M:" << strmn << pAction->objectName() << ptrCurMenu->graph << ptrCurMenu->spidtm << ptrCurMenu->scale << ptrCurMenu->factor << ptrCurMenu->approxim << ptrCurMenu->data.size();

    if(strmn == "" || pAction->objectName() == "" || ptrCurMenu->graph == "" || ptrCurMenu->data.size() < 1 ) return;

    setWindowTitle(strmn);
    refreshData();

    pgrd->update();
    return;
}// End slotMenuClicked

//---------------------------------------------------------------------------

void Monitor::slotTimerRefresh(){
    QDateTime tmpDTime = curDTime;
    curDTime = QDateTime::currentDateTime();       // Время
    if(ptrCurMenu == nullptr) return;

    if((curDTime.time().minute() % 10) == 0 && curDTime.time().second() < 10){	// 10мин

//Количество данных зависит от прошедшего времени !!!
        auto tmax = curDTime.toMSecsSinceEpoch();
        auto tmin = tmpDTime.toMSecsSinceEpoch();
qDebug() << "Monitor::slotTimerRefresh() Get DT:" << tmin << tmax << tmax - tmin;

        refreshData();		// НАДО оптимизировать размер запрашиваемых данных !!!
        pgrd->update();
    }
}// End slot

//---------------------------------------------------------------------------

void Monitor::keyPressEvent(QKeyEvent *event){
//qDebug() << tr("Kl >>") << event->key();
    switch (event->key()) {
    case 16777264:	// F1
        slotActivHelp();
        break;

    case 16777267:	// F4
        slotTimerRefresh();
        pgrd->update();
        break;

    case 16777268:	// F5
        refreshData();
        pgrd->update();
        break;

//    case 16777269:	// F6
    case 16777270:	// F7
    case 16777271:	// F8
        refreshData();
        pgrd->update();
        break;

    case 16777272:	// F9
        pmenu->show();
        break;


    case 16777273:	// F10
    case 16777216:	// esc
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
            "Просмотр графиков<BR>"
            "F5 обновить полностью<BR>"
            "F6 дополнить за прошедшее время<BR>"
            "esc F10 завершить<BR>"
            "<BR>"
            "<BR>"
        "</P>"
        "<H3><CENTER> Версия 0.5 </CENTER></H3>"
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
