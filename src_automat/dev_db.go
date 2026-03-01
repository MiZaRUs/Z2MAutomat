/****************************************************************************
 *     Created  in  2025-2026  by  Oleg Shirokov   oleg@shirokov.online     *
 ****************************************************************************/

package main

import (
    "log"
    "fmt"
    "time"
    "sync"
    "strconv"
    "math"
    "encoding/json"
    "encoding/binary"
    "gopkg.in/yaml.v3"
    "os"
    "../ipc"
)

//----------------------------------------

type Z2MDevice struct {
    FriendlyName string `yaml:"friendly_name"`
    Retain       bool   `yaml:"retain"`
    Qos          byte   `yaml:"qos"`
    Description  string `yaml:"description"`
    HA struct {
        Name    string `yaml:"name"`
    } `yaml:"homeassistant"`
}

type Z2MConfig struct {
    Vers          string `yaml:"version"`
    MQTT  struct {
        Btopic    string `yaml:"base_topic"`
        Server    string `yaml:"server"`
    } `yaml:"mqtt"`
    Devices   map[string]Z2MDevice  `yaml:"devices"`
}

//----------------------------------------

type ZBDev struct {
    mut       sync.RWMutex
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


// * конфигурации устройств загружаем из файла z2m configuration.yaml
// * требуется скорректировать поля секции devices (можно через web-z2m)
// description=название и homeassistant.name=параметры

func loadDevicesConfig(conff string, index map[string]*ZBDev) {
    if index == nil { return }
    yamlFile, err := os.ReadFile(conff)
    if err != nil {
        log.Println("ERROR loadDevicesConfig.ReadFile:", err)
        return
    }

    var z2mconf Z2MConfig
    err = yaml.Unmarshal(yamlFile, &z2mconf)
    if err != nil {
        log.Println("ERROR loadDevicesConfig.Unmarshal:", err)
        return
    }
    log.Printf("Z2MConfig: version:%#v, mqtt_base_topic:%#v, mqtt_server:%#v, devices:%d", z2mconf.Vers, z2mconf.MQTT.Btopic, z2mconf.MQTT.Server, len(z2mconf.Devices))

    tmu := time.Now()
    for key, v := range z2mconf.Devices {
        if key != "" && v.FriendlyName != "" && v.Description != "" {
            exec := false
            if len(v.HA.Name) > 7 && v.HA.Name[:8] == `executor` { exec = true }
            index[key] = &ZBDev{uid:v.FriendlyName, tmup:tmu, executor:exec, qos:v.Qos, Name:v.Description}
            log.Printf("K:%#v  uid:%#v  qos:%d  executor:%#v  name:%#v prm:%#v", key, v.FriendlyName, v.Qos, exec, v.Description, v.HA.Name)
        }
    }
}

//---------------------------------------------------------------------------


// * Обновление состояния устройства.

func (db *service) updateZ2MDevice(uid string, jsmsg []byte) *ZBDev{	// 
    db.mut.Lock()
    defer db.mut.Unlock()
    if dx, ok := db.device_index[uid]; ok && dx != nil {
        dx.mut.Lock()
        defer dx.mut.Unlock()
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
    dev.mut.RLock()
    defer dev.mut.RUnlock()
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
    dev.mut.RLock()
    defer dev.mut.RUnlock()
    if act, ok := dev.status[str]; ok && act != nil {
        switch val := act.(type) {
        case bool:
            return val
        case int:
            if val == 0 { return false } else { return true }
        case float64:
            if val == 0 { return false } else { return true }
        case string:
            if val == "" { return false } else { return true }
        }
    }
    return false
}

func (dev *ZBDev) Int(str string) int {
    if dev.uid == "" || dev.status == nil { return 0 }
    dev.mut.RLock()
    defer dev.mut.RUnlock()
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
            if val == true { return 1 } else { return -1 }
        }
    }
    return 0
}

func (dev *ZBDev) Digit(str string) float64 {
    if dev.uid == "" || dev.status == nil { return 0 }
    dev.mut.RLock()
    defer dev.mut.RUnlock()
    if act, ok := dev.status[str]; ok && act != nil {
        switch val := act.(type) {
        case float64:
            return val
        case int:
            return float64(val)
        case string:
            if v, er := strconv.ParseFloat(val, 64); er == nil { return v }
            if val == "ON" { return 1 } else if val == "OFF" { return -1 }
        case bool:
            if val == true { return 1 } else { return -1 }
//        default:
//            log.Println("::::::::::::::++", act, val, fmt.Sprintf("T:%T", val))
        }
    }
    return 0
}

//---------------------------------------------------------------------------

func (dev *ZBDev) SaveSensors(sens []string) {
    if monitor_addr == "" || dev.uid == "" { return }
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
    if len(data) < 22 || len(data) > 250 { return }
    if er := ipc.SendSHAMsg(monitor_addr, data); er != nil { log.Println("ERROR ipc.SendSHAMsg()", er) }
}

//---------------------------------------------------------------------------

func (dev *ZBDev) SaveExecutorState() {
    if !dev.executor || monitor_addr == "" || dev.uid == "" || dev.status == nil { return }
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
    if er := ipc.SendSHAMsg(monitor_addr, data); er != nil { log.Println("ERROR ipc.SendSHAMsg()", er) }
}

//---------------------------------------------------------------------------
