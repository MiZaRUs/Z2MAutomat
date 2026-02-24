/****************************************************************************
 *     Created  in  2025-2026  by  Oleg Shirokov   oleg@shirokov.online     *
 ****************************************************************************/

package main

import (
    "log"
    "fmt"
    "time"
//    "sync"
//    "strings"
    "strconv"
    "math"
    "encoding/json"
    "encoding/binary"
    "../ipc"
)

//----------------------------------------

type ZBDev struct {
    uid       string
    qos       byte
    Name      string
//    retained  bool
    status    map[string]interface{}	// данные/состояние устройства (всё что есть в json zigbee2mqtt)
    tmup      time.Time			// время обновления данных
    lastst    bool
    executor  bool
}

//---------------------------------------------------------------------------


// * конфигурации устройств (НАДО загружать из файла !!! файл устройств общий для микросервисов


func loadDevicesConfig(conf string, index map[string]*ZBDev) {
    if index == nil { return }
    tmu := time.Now()

// датчики протечки
    index["0xa4c138ade4c67c34"] = &ZBDev{uid:"0xa4c138ade4c67c34", tmup:tmu, qos:2, Name:"Душевая, под умывальником"}		// "tamper":false,"water_leak":false
    index["0xa4c138061ca5ff5a"] = &ZBDev{uid:"0xa4c138061ca5ff5a", tmup:tmu, qos:2, Name:"Туалет, за унитазом"}			// "tamper":false,"water_leak":false
    index["0xa4c1384b234a0c7e"] = &ZBDev{uid:"0xa4c1384b234a0c7e", tmup:tmu, qos:2, Name:"Кухня, под раковиной"}		// "tamper":false,"water_leak":false - плохая батарея !!!

// ручное управление
    index["0x20a716fffef03087"] = &ZBDev{uid:"0x20a716fffef03087", tmup:tmu, qos:2, Name:"Кнопка-1" }		// action: "single", double", "long"
    index["0xa4c1382b7c6b84f5"] = &ZBDev{uid:"0xa4c1382b7c6b84f5", tmup:tmu, qos:2, Name:"Кнопка, столовая" }	// action: "single", double", "hold"

    index["0xa4c138bf239fc880"] = &ZBDev{uid:"0xa4c138bf239fc880", tmup:tmu, qos:2, Name:"Кухня, присутствие"}	// presence: false, true    + Illuminance:int + presence_sensitivity + target_distance + detection_distance_{max|max}
    index["0xa4c138acbd2987a4"] = &ZBDev{uid:"0xa4c138acbd2987a4", tmup:tmu, qos:2, Name:"Кухня, чайный стол"}	// presence: false, true    + Illuminance:int + presence_sensitivity + target_distance + detection_distance_{max|max}
    index["0xa4c1387d9dbc566f"] = &ZBDev{uid:"0xa4c1387d9dbc566f", tmup:tmu, qos:2, Name:"Кухня, активность" }	// occupancy: false, true  + illuminance:int

// климат
    index["0xa4c138ac1692f499"] = &ZBDev{uid:"0xa4c138ac1692f499", tmup:tmu, qos:2, Name:"Кухня"}		// ,"humidity":24.5,"temperature":25.75
    index["0xa4c1388d7520cf68"] = &ZBDev{uid:"0xa4c1388d7520cf68", tmup:tmu, qos:2, Name:"Кабинет"}		// ,"humidity":24.5,"temperature":25.75
    index["0xa4c138d1df3edebd"] = &ZBDev{uid:"0xa4c138d1df3edebd", tmup:tmu, qos:2, Name:"Спальня"}		// ,"humidity":24.5,"temperature":25.75

// executor
    index["0xa4c138e98909dd43"] = &ZBDev{uid:"0xa4c138e98909dd43", executor:true, qos:0, Name:"Кухня - Освещение" }		// "state_l1:"OFF","ON" + "state_l2:"OFF","ON" + "state_l3:"OFF","ON" + "state_l4:"OFF","ON"
    index["0x70b3d52b601780f4"] = &ZBDev{uid:"0x70b3d52b601780f4", executor:true, qos:0, Name:"Кухня - Фонарь" }		// "state":
    index["0xa4c138853d5b9c40"] = &ZBDev{uid:"0xa4c138853d5b9c40", tmup:tmu, executor:true, qos:0, Name:"Кухня - Розетка"}	// "state":"OFF","ON"
}

//---------------------------------------------------------------------------


// * Обновление состояния устройства.

func (db *service) updateZ2MDevice(uid string, jsmsg []byte) *ZBDev{	// 
    db.mut.Lock()
    defer db.mut.Unlock()
    if dx, ok := db.device_index[uid]; ok && dx != nil {
        if _, ok := dx.status["action"]; ok { dx.status["action"] = "" }	// если есть, обнулим прежнее значение !!!

        var tmpstat interface{}		// сохраним прежнее значение
        if x, ok := dx.status["contact"]; ok { tmpstat = x }
        if x, ok := dx.status["occupancy"]; ok { tmpstat = x }
        if x, ok := dx.status["presence"]; ok { tmpstat = x }
        if x, ok := dx.status["water_leak"]; ok { tmpstat = x }

        if err := json.Unmarshal(jsmsg, &dx.status); err == nil {	// если есть изменение - пометим !!!
            dx.tmup = time.Now()
            dx.lastst = false
            if x, ok := dx.status["contact"]; ok && tmpstat != x { dx.lastst = true }
            if x, ok := dx.status["occupancy"]; ok && tmpstat != x { dx.lastst = true }
            if x, ok := dx.status["presence"]; ok && tmpstat != x { dx.lastst = true }
            if x, ok := dx.status["water_leak"]; ok && tmpstat != x { dx.lastst = true }

            if uid == "0x449fdafffe145831" {
                log.Println(" ^^^^^^^ ", dx.Name, string(jsmsg))
            }

            return dx								// вернём с новыми значениями
        }else{
            log.Println("ERROR ZBMSG:", err)
        }
    }else{
        log.Println("WARNING ZBMSG:", uid,"- устройства нет в конфигурации.", string(jsmsg) )
    }
    return nil	// вернуть nil если не требуется исполнение !!!
}

//---------------------------------------------------------------------------

func (dev *ZBDev) String(str string) string {
    if dev.uid == "" || dev.status == nil { return "" }
    if act, ok := dev.status[str]; ok && act != nil {
        switch val := act.(type) {
        case string:
            return val
        case int:
            return fmt.Sprintf("%d", val)	//strconv.Itoa(val)
        case float64:
            return fmt.Sprintf("%f", val)
        default:
            return fmt.Sprint(val)
        }
    }
    return ""
}

func (dev *ZBDev) Bool(str string) bool {
    if dev.uid == "" || dev.status == nil { return false }
    if act, ok := dev.status[str]; ok && act != nil {
        switch val := act.(type) {
        case bool:
            return val
        case int:
            if val == 0 { return false} else { return true }
        case float64:
            if val == 0 { return false} else { return true }
        case string:
            if val == "" { return false} else { return true }
        }
    }
    return false
}

func (dev *ZBDev) Int(str string) int {
    if dev.uid == "" || dev.status == nil { return 0 }
    if act, ok := dev.status[str]; ok && act != nil {
        switch val := act.(type) {
        case int:
            return val
        case float64:
            return int(val)
        case string:
            v,_ := strconv.Atoi(val)
            return v
        case bool:
            if val == true { return 1} else { return 0 }
        }
    }
    return 0
}

func (dev *ZBDev) Digit(str string) float64 {
    if dev.uid == "" || dev.status == nil { return 0 }
    if act, ok := dev.status[str]; ok && act != nil {
        switch val := act.(type) {
        case float64:
            return val
        case int:
            return float64(val)
        case string:
            v,_ := strconv.ParseFloat(val, 64)
            return v
        case bool:
            if val == true { return 1} else { return 0 }
//        default:
//            log.Println("::::::::::::::++", act, val, fmt.Sprintf("T:%T", val))
        }
    }
    return 0
}

//---------------------------------------------------------------------------

func (dev *ZBDev) SaveSensors(sens []string) {
    var data []byte
    data = append(data, 1)							// 1 байт - тип пакета - метрика (1)
    for _, sn := range sens {	// упакуем имя и значения сенсоров
        var bf [16]byte
        binary.BigEndian.PutUint64(bf[:], uint64(dev.tmup.UnixMilli()))		// 8 байт - время
        binary.BigEndian.PutUint64(bf[8:], math.Float64bits(dev.Digit(sn)))	// 8 байт - значение
        data = append(data, bf[:]...)
        data = append(data, []byte(dev.uid+":"+sn)...)				// дополним строкой - uid:сенсор
        data = append(data, 0)							// завершим 0
    }
//    log.Println("SaveSensors()", dev.uid, sens, len(data))
    if dev.uid == "" || len(data) < 22 || len(data) > 250 { return }
    if er := ipc.SendSHAMsg("localhost:10101", data); er != nil { log.Println("ERROR ipc.SendSHAMsg()", er) }
}

//---------------------------------------------------------------------------

func (dev *ZBDev) SaveExecutorStatus() {
    if dev.uid == "" || dev.status == nil { return }
//    log.Println("SaveExecutorStatus():", dev.uid, dev.Name)
    var sensors = []string{"state","state_l1","state_l2","state_l3","state_l4"}	// 1 или 4 реле, до 32
    var res = uint64(0)
    for i, sn := range sensors {
        if act, ok := dev.status[sn]; ok && act != nil {
            switch val := act.(type) {
            case string:
                if val == "OFF" || val == "ON" {
                    res = res|uint64(0x100000000<<i)
                    if val == "ON" { res = res|uint64(1<<i) }
//                    log.Println(" === SaveExecutorStatus():", dev.uid, dev.Name, i, sn, val, fmt.Sprintf("RES:%X", res))
                }
            }
        }
    }
//    log.Println(" === SaveExecutorStatus():", dev.uid, dev.Name, fmt.Sprintf("state:%X", res))
    var data []byte
    data = append(data, 1)						// 1 байт - тип пакета - метрика (1)
    var bf [16]byte
    binary.BigEndian.PutUint64(bf[:], uint64(dev.tmup.UnixMilli()))	// 8 байт - время
    binary.BigEndian.PutUint64(bf[8:], res)				// 8 байт - значение
    data = append(data, bf[:]...)
    data = append(data, []byte(dev.uid+":state")...)			// дополним строкой - uid:сенсор
    data = append(data, 0)						// завершим 0
    if er := ipc.SendSHAMsg("localhost:10101", data); er != nil { log.Println("ERROR ipc.SendSHAMsg()", er) }
}

//---------------------------------------------------------------------------
