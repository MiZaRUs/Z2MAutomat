/****************************************************************************
 *            Created    2026    by    oleg@shirokov.online                 *
 ****************************************************************************/

#include "WxGraph.h"

/*  QColor(200, 0, 0);		// красный
    QColor(0, 122, 0);		// зелёный
    QColor(255, 0, 255);	// сереневый
    QColor(0, 0, 200);		// синий
    QColor(100, 100, 0);	// коричневый
    QColor(0, 165, 155);	// полубой
    QColor(170, 170, 0);	// желтый
*/
// Тест заполнения графика
//    trends.push_back(Trend()); = QColor(200,0,0);
//    trends.push_back(Trend(QColor(0,122,0)));
//    uint zz = 0;
//    for (auto& tr : trends) {
//        for(uint i = 0; i < 24; i++){
//            tr.Points.push_back(QPointF(i, (i+zz)));
//        }
//        zz = zz + 1;
//    }
// ----------------------------------------------------------------------

WxGraph::WxGraph( QWidget* pwgt/*= 0*/) : QWidget(pwgt){
//  шкала времени X
    xmm = 20;		// мелкие деления окна по X
    setPosition(0,0,0); // будут установлены предопределённые значения)

//  шкала данных Y
    ymm = 10;		// мелкие деления окна по Y
    setScale(1);	 // 0...50 & ysm = 5

    show();
}// End WxGrid

// ----------------------------------------------------------------------

void WxGraph::drawGraph(QPainter *painter){
    if(trends.size() < 1) return;
    float yt = (float)yZero / (ysm * 10);	 // деление шкалы Y - масштабируется по окну
    for (auto tr : trends) {
        if(tr.Points.size() < 2) break;
        QPointF tpx[tr.Points.size()];
        for(uint i = 0; i < tr.Points.size(); i++){			// Пересчет координат
            tpx[i].rx() = (float)xZero - (tr.Points[i].rx());		//time
            tpx[i].ry() = (float)yZero - (tr.Points[i].ry() * yt);	//data
        }
        painter->setPen(QPen(tr.Color, 1.2));
        painter->drawPolyline(tpx, tr.Points.size());		// рисуем графики
    }
}// End drawGrid

// ----------------------------------------------------------------------

void WxGraph::drawGrid(QPainter *painter){
    int j = pos;				// сдвиг разметки времени
    if((j < 1 )||(j > xsm)) j = xsm;
    for(int i = xZero; i > 0; i--, j++){	// вертикальные линии привязаны к пикселам
        if(!(j % xmm)){
            if(j % xsm){
                painter->setPen(QPen(Qt::black, 0.1));
            }else{
                painter->setPen(QPen(Qt::black, 0.2));
                j = 0;
            }
            painter->drawLine( i, 0, i, yZero );
        }
    }
    painter->setPen(QPen(Qt::black, 0.9));
    painter->drawLine( xZero, 0, xZero, yZero );
    painter->setPen(QPen(Qt::black, 0.5));
    painter->drawLine( 0, 0, 0, yZero );

    j = ysm * 10;
    float t = (float)yZero / j;
    for(int i = 0; i < j; i++){			// горизонтальные линии масштабируемые
        if(i%10){
            painter->setPen(QPen(Qt::black, 0.1));
        }else{
            painter->setPen(QPen(Qt::black, 0.2));
        }
        painter->drawLine( 0, i*t, xZero, i*t );
    }
    painter->setPen(QPen(Qt::black, 0.5));
    painter->drawLine( 0, 0, xZero, 0 );
    painter->drawLine(0, yZero, xZero, yZero);
}// End drawGrid

// ----------------------------------------------------------------------

void WxGraph::drawScale(QPainter *painter){	// надписи
    painter->setPen(QPen(Qt::darkBlue, 0.7));
    painter->drawText(2, 14, QString::number(scaleMax));
    painter->drawText(2, yZero - 2, QString::number(scaleMin));

    int tm = tmx;					// текушее значение времени для отсчета шкалы
    int j = pos;				// сдвиг разметки времени
    for(int i = 0; i < xZero; i++){
        if(!(i % xsm)){
            painter->drawText((xZero-i-9)+j, yZero - 2, QString::number(tm+1));
            if((tm--) < 1) tm = max;
        }
    }
}// End drawScale

// ----------------------------------------------------------------------

void WxGraph::paintEvent(QPaintEvent* /*event*/){
    QPainter painter( this );
    painter.setRenderHint(QPainter::Antialiasing, true);
    yZero = height();		// высота
    xZero = width();		// ширина
    drawGrid(&painter);
    drawGraph(&painter);
    drawScale(&painter);
}// End paintEvent

// ----------------------------------------------------------------------

void WxGraph::setPosition(int t1, int t2, int mx){	// шкала X - Время
    max = mx;				// 59; //or 23

    tmx = t1;				// время начала шкалы
    xsm = xmm * 6;			// крупные деления, в большенстве случаев кратны 6 !
    pos = xmm * ((60-t2)/10);		// сдвиг разметки времени (десяток минут)

    if(t1 == 0 && t2 == 0 && mx == 0 ){	//предопределённые значения, шкала в часовках
        max = 23;			// 59; //or 23
        tmx = 0;
        pos = xsm;
//        QTime tm = QDateTime::currentDateTime().time();
//        tmx = tm.hour();
//        pos = xmm * ((60-tm.minute())/10);
    }	// часовая шкала
}

// ----------------------------------------------------------------------

void WxGraph::setScale(int sc){			// шкала Y - Данные
    if(sc == 4){	//    0 : 2000
        ysm = 20;
        scaleMax = 2;
        scaleMin = 0;			// подпись )
    }

    if(sc == 3){	//    0 : 1000
        ysm = 10;
        scaleMax = 1;
        scaleMin = 0;			// подпись )
    }

    if(sc == 2){	//    0 : 100
        ysm = 10;
        scaleMax = 100;
        scaleMin = 0;			// подпись )
    }

    if(sc == 1){	//    0 : 50
        ysm = 5;		// деления окна 2...15
        scaleMax = 50;		// QString::number( 50 );	// надпись
        scaleMin = 0;		// подпись )
    }
}// End setScale

// ----------------------------------------------------------------------
