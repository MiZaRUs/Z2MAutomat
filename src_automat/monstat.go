/****************************************************************************
 *     Created  in  2025-2026  by  Oleg Shirokov   oleg@shirokov.online     *
 ****************************************************************************/

package main

import (
    "log"
    "net"
    "time"
)

//----------------------------------------
//---------------------------------------------------------------------------

func (s *service) startTimerW(itm int, name string ) {		// TON/TOF/TP - универсальные таймеры отложенных действий
    s.muttim.Lock()
    defer s.muttim.Unlock()
    if t, ok := s.timer_index[name]; ok && t != nil {
        s.timer_index[name].Stop()
        s.timer_index[name].Reset(time.Second * time.Duration(itm))
    }else{
        s.timer_index[name] = time.NewTimer(time.Second * time.Duration(itm))	// Создаем таймер
    }
    go func() {
        <-s.timer_index[name].C						// Ждем, пока сработает
        s.timer_index[name].Stop()
        s.sensor_event <- &ZBDev{uid:name}				// Сообщение с именем таймера
    }()
}

//---------------------------------------------------------------------------

func (s *service) procStatusMonitor() {       // Мониторинг состояния, с генерацией событий по каким-либо признакам.
    defer log.Printf("ERROR Завершён процесс мониторинга!")

    checkact_ticker := time.NewTicker(time.Minute * 10)		// 10 минутный ТАЙМЕР
    update_ticker   := time.NewTicker(time.Hour)		// часовой ТАЙМЕР
    go func() {
        for range checkact_ticker.C {
            s.messag_event <- 0xFFFFFFF8
        }
    }()
    go func() {
        for range update_ticker.C {
            s.messag_event <- 0xFFFFFFFF
        }
    }()

    log.Printf("Стартуем процесс мониторинга.")
    for {
        time.Sleep(time.Millisecond * time.Duration(10))
        if lvl := <- s.messag_event; lvl > 0 {
            tmnow := time.Now()

            if lvl < 100 { s.checkNotification(tmnow, lvl) }	// level 1...99 - отправить оповещение если есть!


            if lvl == 0xFFFFFFF8 { // каждые 10 минут
                log.Println("#10")
                s.checkOtherDevState(tmnow)			// проверить состояние других устройств (не z2m)
                s.checkNotification(tmnow, 0)			// проверить состояние системы оповещений
            }

            if lvl == 0xFFFFFFFF { // раз в час
                log.Println("#####################    ЧАСОВОЙ ТАЙМЕР    #####################")

                s.mut.RLock()
                for i, d := range s.device_index {          // Проверить обновление данных
                    if tmx := int(tmnow.Sub(d.tmup).Seconds()); tmx >= (3600 * 24 * 2) {    // час*n*s
                        log.Println("WARNING АВАРИЯ", i, d.Name, "TMUP:", tmx, "Нет данных !!!" )           // АВАРИЯ !!!!
                        d.tmup = time.Now()                 // сброс аварии
                    }
                }
                s.mut.RUnlock()
            } // часовой
        } // chan
    } // for безусловный
}

//---------------------------------------------------------------------------

func (s *service) checkOtherDevState(tmnow time.Time){		// проверить состояние других устройств (не z2m)
    var dev *ZBDev
    s.mut.RLock()
    if dx, ok := s.device_index[nameActivityInTheKitchen]; ok && dx != nil { dev = dx }
    s.mut.RUnlock()
    if dev == nil || dev.status == nil { return }
    log.Println(" *************** check ActivityInTheKitchen.State:", len(dev.status))

    state := "OFF"
    stx := false
    if dev.String("tvtemp") == "ON" { stx = true }	// предыдущий и свежий
    if checkTV("192.168.0.88") { state = "ON" }		// долгая функция!
    dev.setString("tvtemp", state)
    if !stx { state = "OFF" }
    dev.setString("TV", state)


    for k,v := range dev.status {
        if k != "tvtemp" { log.Println(" --- status Kitchen:", k, v) }
    }


    if dev.String("Sb") == "X" && dev.String("TV") == "ON" {		// никого нет, а телек включен !!!
        log.Println(" -------------- НАДО ВЫКЛЮЧИТЬ ТЕЛЕВИЗОР !!!")
//        s.executeTVOFF()
    }
}

//---------------------------------------------------------------------------

// порты бывают доступны в выключенном состоянии-ожидании AndroidTV.
func checkTV(ip string) bool {	// проверка доступности портов AndroidTV
    if con, err := net.DialTimeout("tcp", ip+":8008", time.Duration( time.Second * 3 )); err == nil {
        con.Close()
        return true
    } else if con, err := net.DialTimeout("tcp", ip+":8009", time.Duration( time.Second * 3 )); err == nil {
        con.Close()
        return true
    }
    return false
}

//---------------------------------------------------------------------------
