/****************************************************************************
 *     Created  in  2025-2026  by  Oleg Shirokov   oleg@shirokov.online     *
 ****************************************************************************/

package main

import (
    "log"
    "fmt"
    "net"
//    "bytes"
    "time"
    "runtime/debug"
//    "encoding/binary"
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

func (s *service) executeTVOFF() {      // Требуется предварительная синхронизация s.mut.R
    if dx, ok := s.device_index[nameActivityInTheKitchen]; ok && dx != nil && dx.String("TV") == "ON" {
        for i:=0; i < 3; i++ {
            if checkTV("192.168.0.88") {
                s.publish(Z2M+"0x449fdafffe145831/set", 0, false, `{"ir_code_to_send":"BW8jsxFHAsABA50GRwLgCwFAF0ADQAFAB+AHA+ADAUAb4AcBQBPAA0ABwAvABwkKom8jBQlHAv//4DoHAglHAg=="}`)             // ON/OFF HaierTV
                s.publish(Z2M+"0x449fdafffe145831/set", 0, false, `{"ir_code_to_send":"BW8jsxFHAsABA50GRwLgCwFAF0ADQAFAB+AHA+ADAUAb4AcBQBPAA0ABwAvABwkKom8jBQlHAv//4DoHAglHAg=="}`)             // ON/OFF HaierTV
            } else {
                log.Println(" * TV: OFF")
                break
            }
            time.Sleep(time.Second * time.Duration(40))
        }
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

func (s *service) procTaskControl() {       //  Управление задачами, требующие анализа данных и сложной логики
    var sched_event = make(chan uint64)				// Событие-извещение
    ten_minutes_ticker := time.NewTicker(time.Minute * 10)	// 10 минутный ТАЙМЕР
    every_hour_ticker  := time.NewTicker(time.Hour)		// часовой ТАЙМЕР

    go func() {
        for range ten_minutes_ticker.C {
            sched_event <- 0xFFFFFFF8
        }
    }()
    go func() {
        for range every_hour_ticker.C {
            sched_event <- 0xFFFFFFFF
        }
    }()

    log.Printf("Стартуем процесс контроля заданий.")
    go func() {
        defer s.recoveryTaskControl()			// перезапустим при неожиданных сбоях
        for {
            time.Sleep(time.Millisecond * time.Duration(10))
            if lvl := <- sched_event; lvl > 0 {		// синхронизируем последовательность таймеров
                tmnow := time.Now()

                if lvl == 0xFFFFFFF8 { // каждые 10 минут
                    log.Println("#10")
                    s.checkOtherDevState(tmnow)			// проверить состояние других устройств (не z2m)
                }

                if lvl == 0xFFFFFFFF { // раз в час
                    log.Println("#####  ЧАСОВОЙ ТАЙМЕР  ####")

                    s.mut.RLock()
                    for i, d := range s.device_index {          // Проверить обновление данных
                        if tmx := int(tmnow.Sub(d.tmup).Seconds()); tmx >= (3600 * 24 * 2) {    // час*n*s
                            s.notification.Send(d.tmup, "Внимание!", fmt.Sprintf("Устройство %s не активно! %s", d.Name, d.uid))
                            log.Println("WARNING АВАРИЯ", i, d.Name, "TMUP:", tmx, "Нет данных !!!" )           // АВАРИЯ !!!!
                            d.tmup = time.Now()                 // сброс аварии
                        }
                    }
                    s.mut.RUnlock()
                } // часовой
            } // chan
        } // for безусловный
    }()
}

//---------------------------------------------------------------------------

func (s *service) recoveryTaskControl() {
    log.Printf("ERROR Завершён процесс контроля заданий!")
    if recoveryMessage := recover(); recoveryMessage != nil {
        log.Println("recoveryTaskControl():", recoveryMessage, string(debug.Stack()), "\n****\n")
    }
}

//---------------------------------------------------------------------------
