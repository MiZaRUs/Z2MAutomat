/****************************************************************************
 *            Created    2026    by    oleg@shirokov.online                 *
 ****************************************************************************/

#ifndef _Monitor_h_
#define _Monitor_h_

#include <QtWidgets>
#include "WxGraph.h"

//---------------------------------------------------------------------------

struct GrData {
    QString uid;
    QString sensor;
    QString title;
    QColor color;
};

struct WxMenu {
    QString graph;
    int spidtm;
    int scale;
    double factor;
    std::vector<GrData> data;
};

struct AppConf {
    QString url;
    std::vector<WxMenu> menu;
};

//---------------------------------------------------------------------------

class Monitor : public QWidget {

    Q_OBJECT

public:
    explicit Monitor(AppConf*, QWidget *parent = nullptr);
    double getData(int n, QString uid, QString sensor, qint64 tmin, qint64 tmax );

private:
    QTimer *timer;
    QVBoxLayout *plout;
    QMenu *pmenu;
    WxGraph *pgrd;

    QString url;
    std::map<QString, WxMenu> config;

    QDateTime curDTime;

    float xfactor;	// множитель шкалы X
    float yfactor;	// множитель шкалы Y

//    void refreshDate(void);

protected:
    virtual void contextMenuEvent(QContextMenuEvent *pe){
	pmenu->exec(pe->globalPos());
    }
    virtual void keyPressEvent(QKeyEvent *event);

public slots:
    void slotMenuClicked(QAction *pAction);
    void slotActivExit();
    void slotActivHelp();
    void slotTimerRefresh();
};

//---------------------------------------------------------------------------

#endif  //_Monitor_h_
