/****************************************************************************
 *            Created    2026    by    oleg@shirokov.online                 *
 ****************************************************************************/

#ifndef _WxGraph_h_
#define _WxGraph_h_

#include <QtWidgets>
//#include <vector>

//---------------------------------------------------------------------------

class Trend {
public:
    explicit Trend(QColor c = QColor(170,170,0)) {		// по умолчанию жёлтый!
        Color = c;
    }
    QColor Color;
    std::vector<QPointF> Points;
};

//---------------------------------------------------------------------------

class WxGraph: public QWidget {

public:
    explicit WxGraph(QWidget *parent = nullptr);

    std::vector<Trend> trends;

    void ClearPoints(void){ for(auto& tr : trends) { tr.Points.clear(); }}         // Очисть графики
    void setScale(int);
    void setPosition(int, int, int);
    int getLenXScale(){ return xZero/xmm; }

private:
    int xmm;		// делений на шкале "милиX"
    int xsm;		// разделы "сантиX"
    int pos;		// позиция диаграммы по времени
    int tmx;		// начальное значение отсчета времени
    int max;		// предельное значение отсчета времени 59 или 23

    int ymm;		// делений на шкале "милиY"
    int ysm;		// разделы "сантиY"
    int scaleMax, scaleMin;	// значения шкалы Y. (QString::number(50))

    int yZero, xZero;	// нижний и правый край окна

    void drawGrid(QPainter *painter);	// сетка
    void drawGraph(QPainter *painter);	// графики
    void drawScale(QPainter *painter);	// надписи

protected:
    void paintEvent(QPaintEvent *event);
};

//---------------------------------------------------------------------------

#endif //_WxGraph_h_
