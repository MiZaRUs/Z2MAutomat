/****************************************************************************
 *     Created  in  2025-2026  by  Oleg Shirokov   oleg@shirokov.online     *
 ****************************************************************************/

package main

import (
    "log"
    "fmt"
//    "net"
    "time"
    "strings"
)
//----------------------------------------
const (
    tmsActivityInTheKitchen = 300	// Секунды ожидания прекращения активности на кухне (900)
    nameActivityInTheKitchen = `timerActivityInTheKitchen`		// имя таймера общее для кухни
    nameActivityInTheTeaTable = `0xa4c138e98909dd43#{"state_l1":"OFF"}`	// имя # команда таймера для чайного столика
)

//---------------------------------------------------------------------------

func (s *service) startTimerW(itm int, name string ) {		// TON/TOF/TP - универсальные таймеры отложенных действий
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

func (s *service) executeSetDefault(){	// -- установить начальное состояние !
    s.startTimerW(tmsActivityInTheKitchen, nameActivityInTheKitchen)	// Timer ожидания прекращения активности на кухне - должен стартовать сразу
// НАДО взять из конфига начальные значения для set default ???
}

//---------------------------------------------------------------------------

func (s *service) executeRules(sev *ZBDev) {	// стартует для каждого события !!!
    s.mut.Lock()
    defer s.mut.Unlock()

// проверить состояние батареек и расстояние:  sev.Digit("battery")  +  sev.Bool("battery_low")  +  sev.Digit("linkquality")
    if !sev.executor && sev.Bool("battery_low") {	// только для сенсоров с батарейным питанием !sev.executor
        log.Println("WARNING Требуется замена:", sev.uid, sev.Name, "Батарейка:", sev.Digit("battery"), "%" )
    }
    if d := sev.Digit("battery"); !sev.executor && d > 0 && d < 50 {
        log.Println("WARNING Низкий заряд:", sev.uid, sev.Name, "Батарейка:", d, "%" )
    }
    if d := sev.Digit("linkquality"); d > 0 && d < 60  {		// МОЖНО сохранить в БД
        log.Println("WARNING Датчик плохо слышно:", sev.uid, sev.Name, d )
    }


    switch sev.uid {
    case "0xa4c138ade4c67c34", "0xa4c1384b234a0c7e", "0xa4c138061ca5ff5a":	// Протечка !!! // "tamper":false,"water_leak":false
        if sev.Bool("water_leak") {
            go s.sendNotification(1, sev.tmup, fmt.Sprintf("АВАРИЯ:ПРОТЕЧКА! %s", sev.Name))
            log.Println("WARNING АВАРИЯ:", sev.uid, sev.Name, "ПРОТЕЧКА!")
        }


    case "0xa4c138ac1692f499", "0xa4c1388d7520cf68", "0xa4c138d1df3edebd" :	// климат ,"humidity":24.5,"temperature":25.75
        sev.SaveSensors([]string{"temperature","humidity"})			// сохранить в БД (длительное хранение)
//        log.Println("Климат:", sev.uid, sev.Name, " Влажность:", sev.Digit("humidity"), " Температура:", sev.Digit("temperature") )


    case "0x20a716fffef03087":		// Кнопка-1 - на холодильнике
        switch sev.String("action") {
        case "single":
            s.startTimerW(360, nameActivityInTheTeaTable)	// Timer - действие отложенное на 360 сек
            if dx, ok := s.device_index["0xa4c138e98909dd43"]; ok && dx != nil && dx.uid != "" && dx.executor {	// реле 4 шт.
                if dx.String("state_l1") == "" || dx.String("state_l1") == "OFF" { s.publish(Z2M+dx.uid+"/set" , dx.qos, false, `{"state_l1":"ON"}`) } else { s.publish(Z2M+dx.uid+"/set" , dx.qos, false, `{"state_l1":"OFF"}`) } // trigger
            }
            if dx, ok := s.device_index["0xa4c138853d5b9c40"]; ok && dx != nil && dx.executor && dx.uid != "" && dx.String("state") != "ON" {	// Розетка. Кухня - Разделочный стол.
                s.publish(Z2M+dx.uid+"/set", dx.qos, false, `{"state":"ON"}`)
            }
        case "double":
            log.Println("WARNING Кнопка:", sev.uid, "Разрешить автоматику!")
            s.automatic = true
        case "long":
            log.Println("WARNING Кнопка:", sev.uid, "Погасить ВСЁ! и Запретить автоматику!")
            s.automatic = false
            go s.executeAllOFF()
        }


    case "0xa4c1382b7c6b84f5":		// Кнопка, столовая
        switch sev.String("action") {
        case "single":
            if d1, ok := s.device_index["0xa4c138e98909dd43"]; ok && d1 != nil && d1.uid != "" && d1.executor {	// реле 4 шт. Освещение столовой
                if d2, ok := s.device_index["0x70b3d52b601780f4"]; ok && d2 != nil && d2.uid != "" && d2.executor  {	// реле - выключатель. Фонарь
                    if d1.String("state_l2") != "ON" && d2.String("state") != "ON" {		// обе отключены
                        s.publish(Z2M+d1.uid+"/set" , d1.qos, false, `{"state_l2":"ON"}`)	// включаем Освещение столовой
                    } else if d1.String("state_l2") == "ON" && d2.String("state") != "ON" {	// столовая
                        s.publish(Z2M+d2.uid+"/set" , d2.qos, false, `{"state":"ON"}`)		// включаем фонарь
                    } else if d1.String("state_l2") == "ON" && d2.String("state") == "ON" {
                        s.publish(Z2M+d1.uid+"/set" , d1.qos, false, `{"state_l2":"OFF"}`)	// отключаем Освещение столовой
                    } else {
                        s.publish(Z2M+d1.uid+"/set" , d1.qos, false, `{"state_l2":"OFF"}`)	// отключаем Освещение столовой
                        s.publish(Z2M+d2.uid+"/set" , d2.qos, false, `{"state":"OFF"}`)		// отключаем фонарь
                    }
        	}
            }
        case "double":
            if dx, ok := s.device_index["0xa4c138e98909dd43"]; ok && dx != nil && dx.uid != "" && dx.executor && dx.String("state_l2") != "ON" {	// реле 4 шт.
                s.publish(Z2M+dx.uid+"/set" , dx.qos, false, `{"state_l2":"ON"}`)
            }
            if dx, ok := s.device_index["0x70b3d52b601780f4"]; ok && dx != nil && dx.uid != "" && dx.executor && dx.String("state") != "ON" {	// реле - выключатель. Фонарь
                s.publish(Z2M+dx.uid+"/set" , dx.qos, false, `{"state":"ON"}`)
            }
        }



    if !s.automatic { return } // Запрет автоматики!


// Кухня
    case "0xa4c138bf239fc880":		// Кухня - Присутствие (микроволновый) presence: false, true    + Illuminance:int + presence_sensitivity + target_distance + detection_distance_{max|max}
        noff := false
        if sev.Bool("presence") {						// Проверяем датчик присутствия
            s.startTimerW(tmsActivityInTheKitchen, nameActivityInTheKitchen)	// Timer - продлеваем отложенное действие
            if dx, ok := s.device_index["0xa4c138e98909dd43"]; ok && dx != nil && dx.executor && dx.uid != "" && dx.String("state_l4") != "ON" && sev.Int("illuminance") < 2 {	// реле 4 шт. Кухня - ночник
                s.publish(Z2M+dx.uid+"/set", dx.qos, false, `{"state_l4":"ON"}`)
            }
            if dx, ok := s.device_index["0xa4c138853d5b9c40"]; ok && dx != nil && dx.executor && dx.uid != "" && dx.String("state") != "ON" {	// Розетка. Кухня - Разделочный стол.
                s.publish(Z2M+dx.uid+"/set", dx.qos, false, `{"state":"ON"}`)
            }

        } else if sev.lastst {		// смена состояния
            log.Println(" --- Кухня:", sev.uid, sev.Int("illuminance"), sev.Int("target_distance"), sev.Int("detection_distance_max") )
            s.startTimerW(tmsActivityInTheKitchen, nameActivityInTheKitchen)	// Timer - продлеваем отложенное действие
            if s2, ok := s.device_index["0xa4c1387d9dbc566f"]; ok && s2 != nil && s2.uid != "" && !s2.Bool("occupancy") {	// никто не маячит на входе
                noff = true
            }
        }
//       if (sev.Int("detection_distance_max") - sev.Int("target_distance")) > 70 	// "target_distance":289 - проверять  "detection_distance_max":490
        if sev.Int("illuminance") > 9 { noff = true }

        if dx, ok := s.device_index["0xa4c138e98909dd43"]; ok && dx != nil && dx.executor && dx.uid != "" && noff && dx.String("state_l4") != "OFF" {	// реле 4 шт. Кухня - ночник
            s.publish(Z2M+dx.uid+"/set", dx.qos, false, `{"state_l4":"OFF"}`)
        }

        if sev.lastst && sev.Int("illuminance") > 1000 {		// Выключить фонарь и освещение столовой !!!
            if dx, ok := s.device_index["0xa4c138e98909dd43"]; ok && dx != nil && dx.uid != "" && dx.executor && dx.String("state_l2") != "OFF" {	// реле 4 шт.
                s.publish(Z2M+dx.uid+"/set" , dx.qos, false, `{"state_l2":"OFF"}`)
            }
            if dx, ok := s.device_index["0x70b3d52b601780f4"]; ok && dx != nil && dx.uid != "" && dx.executor && dx.String("state") != "OFF" {	// реле - выключатель. Фонарь
                s.publish(Z2M+dx.uid+"/set" , dx.qos, false, `{"state":"OFF"}`)
            }
        }
        sev.SaveSensors([]string{"illuminance"})	// сохранить в БД (временное хранение)



// Чайный стол
    case "0xa4c138acbd2987a4":		// Чайный стол - Присутствие (микроволновый) presence: false, true    + Illuminance:int + presence_sensitivity + target_distance
        if sev.Bool("presence") && sev.Int("illuminance") < 1000 && sev.Int("target_distance") < 130 { // && sev.lastst 			// Проверяем датчик присутствия
            log.Println(" + Чайный стол:",sev.uid, sev.Bool("presence"), sev.lastst, sev.Int("illuminance"), " Distance:",sev.Int("target_distance") )
            if dx, ok := s.device_index["0xa4c138e98909dd43"]; ok && dx != nil && dx.uid != "" && dx.executor && dx.String("state_l1") != "ON" {	// реле 4 шт. Чайный стол и Столовая - освещение Смотрим статус исполнителя (ON|OFF)
                s.startTimerW(120, nameActivityInTheTeaTable)	// Timer - действие отложенное на 120 сек
                s.publish(Z2M+dx.uid+"/set", dx.qos, false, `{"state_l1":"ON"}`)
            }
        } else if sev.lastst {		// смена состояния
            log.Println(" - Чайный стол:",sev.uid, sev.Bool("presence"), sev.lastst, sev.Int("illuminance"), " Distance:",sev.Int("target_distance") )
            s.startTimerW(90, nameActivityInTheTeaTable)			// Timer - действие отложенное на 90 сек
            s.startTimerW(tmsActivityInTheKitchen, nameActivityInTheKitchen)	// Timer - продлеваем отложенное действие
        }
        sev.SaveSensors([]string{"illuminance"})	// сохранить в БД для анализа (временное хранение)




// Кухня активность
    case "0xa4c1387d9dbc566f":		// Кухня, столовая - Активность (ПИР) + освещённость ( occupancy illuminance )
        if sev.Bool("occupancy") {					// Проверяем датчик присутствия
            log.Println(" *", sev.Name, sev.uid, sev.lastst, sev.Int("illuminance") )
            s.startTimerW(tmsActivityInTheKitchen, nameActivityInTheKitchen)	// Timer - продлеваем отложенное действие
            if dx, ok := s.device_index["0xa4c138e98909dd43"]; ok && dx != nil && dx.uid != "" && dx.executor && dx.String("state_l1") == "ON" { // реле 4 шт. Чайный стол и Столовая
                if a := sev.Int("illuminance"); a < 2200 {		// освещение недостаточное
                    s.startTimerW(90, nameActivityInTheTeaTable)	// Timer - продлеваем отложенное действие
                } else { log.Println("WARNING illuminance > 2200 :", sev.Name,  a ) }
            }
        }
        sev.SaveSensors([]string{"illuminance"})	// сохранить в БД для анализа (временное хранение)



    case nameActivityInTheKitchen:	// завершения таймера активности на кухне
            log.Println(" <<<[Х]>>> Кухня: давно нет активности!")
            s.turnOffKitchen()		// выключить всё на кухне !!!


    default:	// Проверим наличие команды - событие таймера
        if lid := strings.Split(sev.uid, `#`); len(lid)==2 && len(lid[0])==18 && len(lid[1])>7 {	// внутренние события (таймер...)
            if prm := strings.Split(lid[1], `:`); len(prm)==2 && len(prm[0])>5 && len(prm[1])>0 {	// разберём параметры
                state := prm[0][2:len(prm[0])-1]
                if dx, ok := s.device_index[lid[0]]; ok && dx != nil && dx.uid != "" && dx.executor && dx.String(state) != "OFF" {	// Смотрим статус исполнителя (ON|OFF)
                    s.publish(Z2M+dx.uid+"/set", dx.qos, false, lid[1])
                }
            }
        }
    } // switch
}

//---------------------------------------------------------------------------

func (s *service)turnOffKitchen() {
    if dx, ok := s.device_index["0xa4c138e98909dd43"]; ok && dx != nil && dx.uid != "" && dx.executor  && dx.String("state_l1") != "OFF" {	// реле 4 шт. освещение чайного стола
        s.publish(Z2M+dx.uid+"/set" , dx.qos, false, `{"state_l1":"OFF"}`)
    }
    if dx, ok := s.device_index["0xa4c138e98909dd43"]; ok && dx != nil && dx.uid != "" && dx.executor  && dx.String("state_l2") != "OFF" {	// реле 4 шт. освещение столовой
        s.publish(Z2M+dx.uid+"/set" , dx.qos, false, `{"state_l2":"OFF"}`)
    }
    if dx, ok := s.device_index["0xa4c138e98909dd43"]; ok && dx != nil && dx.uid != "" && dx.executor  && dx.String("state_l3") != "OFF" {	// реле 4 шт. резерв
        s.publish(Z2M+dx.uid+"/set" , dx.qos, false, `{"state_l3":"OFF"}`)
    }
    if dx, ok := s.device_index["0xa4c138e98909dd43"]; ok && dx != nil && dx.uid != "" && dx.executor  && dx.String("state_l4") != "OFF" {	// реле 4 шт. ночник
        s.publish(Z2M+dx.uid+"/set" , dx.qos, false, `{"state_l4":"OFF"}`)
    }
    if dx, ok := s.device_index["0x70b3d52b601780f4"]; ok && dx != nil && dx.uid != "" && dx.executor && dx.String("state") != "OFF" {	// реле - выключатель. Фонарь
        s.publish(Z2M+dx.uid+"/set" , dx.qos, false, `{"state":"OFF"}`)
    }
    if dx, ok := s.device_index["0xa4c138853d5b9c40"]; ok && dx != nil && dx.executor && dx.uid != "" && dx.String("state") != "OFF" {	// Розетка. Кухня - Разделочный стол.
        s.publish(Z2M+dx.uid+"/set", dx.qos, false, `{"state":"OFF"}`)
    }
}

//---------------------------------------------------------------------------

func (s *service) executeAllOFF() {
    log.Println(" * требуется исполнить команду All OFF")//, dev.Type,":", dev.Ptrs)
    for _, dev := range s.device_index {
        if dev.executor && dev.uid != "" && dev.String("state")    != "OFF" { s.publish(Z2M+dev.uid+"/set" , dev.qos, false, `{"state":"OFF"}`) }
        if dev.executor && dev.uid != "" && dev.String("state_l1") != "OFF" { s.publish(Z2M+dev.uid+"/set" , dev.qos, false, `{"state_l1":"OFF"}`) }
        if dev.executor && dev.uid != "" && dev.String("state_l2") != "OFF" { s.publish(Z2M+dev.uid+"/set" , dev.qos, false, `{"state_l2":"OFF"}`) }
        if dev.executor && dev.uid != "" && dev.String("state_l3") != "OFF" { s.publish(Z2M+dev.uid+"/set" , dev.qos, false, `{"state_l3":"OFF"}`) }
        if dev.executor && dev.uid != "" && dev.String("state_l4") != "OFF" { s.publish(Z2M+dev.uid+"/set" , dev.qos, false, `{"state_l4":"OFF"}`) }
    }
//    s.executeTVoff()	// Выключить телевизор, если доступен!
}

//---------------------------------------------------------------------------
