/****************************************************************************
 *     Created  in  2025-2026  by  Oleg Shirokov   oleg@shirokov.online     *
 ****************************************************************************/

package main

import (
    "log"
//    "fmt"
    "net"
    "time"
    "strings"
    "encoding/json"
    "encoding/binary"
    "go.etcd.io/bbolt"
    "net/http"
    "golang.org/x/net/websocket"
    "io/ioutil"
    "os"
//    "sync"
    "runtime/debug"
    "../ipc"
)

// ----------------------------------------
const (
    VERSION = "0.4"
)
// ----------------------------------------

type SM struct {                   // система мониторинга
//    mut        sync.RWMutex
    mdb       *bbolt.DB
    Socket     string
    HttpServ   string
    DebugLevel    int
    MainLoopDelay int
}

//---------------------------------------------------------------------------

func (*SM) recoveryService() { // про сбоях в работе
    if recoveryMessage := recover(); recoveryMessage != nil {
        log.Println("Service.Recovery:", recoveryMessage, "\n", string(debug.Stack()), "\n****\n")
            return
    }
} // defer sm.recoveryService() // защитимся от падений

//---------------------------------------------------------------------------

func (sm *SM) Closer() { // освободить любые выделенные ресурсы!
    log.Println("Завершаем работу сервиса!")
    sm.mdb.Close()
}

//---------------------------------------------------------------------------

func (sm *SM) executeConnect(conn *net.UDPConn, addr *net.UDPAddr, buf []byte) { // для любых SVD сообщений
    dl := int(binary.LittleEndian.Uint16(buf))      // читаем размер данных  + CRC
    buf = buf[2:]                                   // уберём длину
    if (dl > 2) && (dl < 0x0FF) && dl == len(buf) { // заголовок + сообщение и КС
        ksx := uint16(buf[len(buf)-1]) << 8
        ksx += uint16(buf[len(buf)-2])
        if ksx == ipc.CheckSumCRC16_CCITT(buf[:len(buf)-2]) { // проверка CRC !!!
            if buf[0] == 1 && len(buf) > 33 {			// тип пакета - метрики
                sm.saveMetrics(buf[1:len(buf)-2])
            }
        } else {
            log.Printf("SRV CRC: failed! %X", ksx)
        }
    } // len OK
}

//---------------------------------------------------------------------------

//  Обработка запросов WebSocket
func (sm *SM) wsHandler(w *websocket.Conn) {
    var buf = make([]byte, 32)
    var n int
    n, _ = w.Read(buf)
    msg := strings.Split(string(buf[:n]), ":" )

    log.Println("WS:", msg)
    w.Write([]byte("{}"))
}

//---------------------------------------------------------------------------

//  Обработка запросов REST Metrics
func (sm *SM)  restMetrics(w http.ResponseWriter, r *http.Request) {
//    log.Println("restMetrics RequestURI:", string(r.RequestURI))
    w.Header().Set("Content-type", "application/json")
    if r.RequestURI == `/rest/metrics`{
        body, _ := ioutil.ReadAll(r.Body)
//        log.Println("restMetrics ReadAll:", string(body))
        if len(body) > 40 && len(body) < 256 {
//            log.Println("REST:", string(body)) // {"uid":"0xa4c138bf239fc880","sensor":"illuminance", "tmin":1770799588725815806, "tmax":1770803188725815806}
            type REQ struct {
                UID     string `json:"uid"`
                Sensor  string `json:"sensor"`
                TMin    uint64 `json:"tmin"`
                TMax    uint64 `json:"tmax"`
            }
            req := REQ{}
            if err := json.Unmarshal(body, &req); err == nil && req.UID != "" && req.Sensor != "" {
                w.Write([]byte(sm.getMetricsJSON(req.UID, req.Sensor, req.TMin, req.TMax)))
                return
	    }
        }
    }
    w.Write([]byte(`{}`))
}

//---------------------------------------------------------------------------

//  Обработка других запросов
func (sm *SM)  rootHandler(w http.ResponseWriter, r *http.Request) {	//  Обработчик подключившихся клиентов
    log.Println("ERROR RequestURI:", string(r.RequestURI))
    w.Header().Set("Content-type", "application/json")
    w.Write([]byte(`{"error":"ERROR"}`))
}

//---------------------------------------------------------------------------

func (sm *SM) httpService() {		// http service
    log.Printf("Стартуем webService API.")
    defer log.Printf("ERROR Завершён webService API!")

//  -- Старт http сервер ------------------
//    fs := http.FileServer(http.Dir("./pub"))
//    http.Handle("/pub/", http.StripPrefix("/pub/", fs))

    http.Handle("/ws", websocket.Handler(sm.wsHandler))
    http.HandleFunc("/", sm.rootHandler)
    http.HandleFunc("/rest/metrics", sm.restMetrics)

    err := http.ListenAndServe(sm.HttpServ, nil)
    if err != nil {
        log.Fatalf("ListenAndServe: %v", err)
    }
}

//---------------------------------------------------------------------------

//===========================================================================
func main() {
    log.SetFlags(log.Ldate | log.Ltime)
    defer log.Println("WARNING Работа сервиса прекращена!\n\n\n")

    err := os.MkdirAll("./host/data", 0777)
    if err != nil && !os.IsExist(err) {
        log.Println("ERROR MkDir data:", err)
        return
    }

    var sm = SM{Socket:"10101", HttpServ:":8000", DebugLevel:9, MainLoopDelay:10}
    defer sm.recoveryService()
    log.Println("Создаём хранилище")
    if sm.mdb, err = bbolt.Open("./host/data/sm.db", 0600, &bbolt.Options{Timeout: 2 * time.Second}); err != nil {
        log.Println("FATAL_ERROR CreateSMDB.Open:", err)
        return
    }
    log.Println("  SMDB Path:", sm.mdb.Path(), " Stats:", sm.mdb.Stats())

    defer sm.Closer() // не забыть освободить ресурсы )

// НАДО убедится что всё в норме !!!

    go sm.checkMDBStatus()		// Мониторинг состояния хранилища

    go sm.httpService()			// внешний API-сервис

//  -- Старт сервис ----------------
    log.Println("Стартуем сервис. Версия:", VERSION, " Порт:", sm.Socket, " Уровень отладки:", sm.DebugLevel, " Основной цикл:", sm.MainLoopDelay)
    if srvAddr, err := net.ResolveUDPAddr("udp", `:`+sm.Socket); err != nil {
        log.Println("ERROR ListenUDP: неправильный порт слушателя:", err.Error())
    } else {
        srvConn, err := net.ListenUDP("udp", srvAddr)
        if err != nil {
            log.Println("ERROR ListenUDP: невозможно стартовать слушателя:", err.Error())
            return
        }
        defer srvConn.Close()
        log.Println("Слушаем UDP Socket:", sm.Socket)
        for {
            buf := make([]byte, 0xFF) // создаём буфер под запрос 255 байт
            cnt, addr, err := srvConn.ReadFromUDP(buf) // читаем всё
            if err == nil && addr != nil && cnt > 7 && string(buf[:2]) == `sh` && (addr.IP.String() == "localhost" || addr.IP.String() == "127.0.0.1") {
                go sm.executeConnect(srvConn, addr, buf[2:cnt])
            } else if err != nil { // if Read sw
                log.Println("ERROR ReadFromUDP:", cnt, addr, err)
            } //  err != io.EOF
        } // безусловный for
    } // if resolve
} // end
//===========================================================================
