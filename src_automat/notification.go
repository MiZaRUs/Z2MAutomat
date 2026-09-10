/****************************************************************************
 *      Created  in  2025-2026  by  Oleg Shirokov   olgshir@gmail.com       *
 ****************************************************************************/

package main

import (
    "log"
    "fmt"
    "time"
    "encoding/json"
    "encoding/binary"
    "go.etcd.io/bbolt"
    "ipc"
    "bytes"
//    "net/http"
//    "io/ioutil"
    "os"
    "runtime/debug"
    "context"
    firebase "firebase.google.com/go"
    "firebase.google.com/go/messaging"
    "google.golang.org/api/option"
)

//----------------------------------------

type MESSAGE struct {
    tm   time.Time
    tag  string
    msg  string
}

type SECRET struct {    // данные для подключения к сервисам оповещения (FCM, telegram, jabber ... )
    FCMFile   string	`json:"fcm_file"`
    FCMTopic  string	`json:"fcm_topic"`
//    TgToken   string	`json:"tg_token"`
//    TgChatID  string	`json:"tg_chatid"`
}

type NOTIFICATION struct {
    FCMOption option.ClientOption
    secret    SECRET			// данные для подключения к внешним сервисам
    storage   *bbolt.DB			// хранилище всех оповещений
    messag_event  chan MESSAGE		// Событие-извещение для отправителя сообщений
}

const SZ_EVENT_CHAN = 7	// по количеству возможных одновременных сообщений

//----------------------------------------
//tag   -  Bucket
//tmu   -  key
//msg   -  val
//---------------------------------------------------------------------------

func (nn *NOTIFICATION) Send(tm time.Time, tag, msg string){	// Оповещения		Информация, Внимыние!, АВАРИЯ!
    if nn.messag_event != nil { nn.messag_event <- MESSAGE{tm:tm, tag:tag, msg:msg} }	// упорядочить отправку опвещений - буфферизированный канал
}

//---------------------------------------------------------------------------

func (nn *NOTIFICATION) Create( pat string){
    log.Println("Создаём менеджер оповещений:"+pat+"/queue.db")
    if jsf, err := os.ReadFile(pat+"/secret.json"); err != nil || json.Unmarshal(jsf, &nn.secret) != nil {
        nn.secret = SECRET{}
        log.Println("WARNING Ошибка файла secret.json:", err)
    } else if nn.secret.FCMFile != "" {
        nn.FCMOption = option.WithCredentialsFile(pat+"/"+nn.secret.FCMFile)
    }
    if len(nn.secret.FCMFile) < 14 || len(nn.secret.FCMTopic) < 3 {  // Проверить конфигурацию оповещений
        log.Println("\n*********************************************************************\n \t\t\t\tНет информации для оповещателя ! \n*********************************************************************")
        return
    }

    if db, err := bbolt.Open(pat+"/message.db", 0600, &bbolt.Options{Timeout: 2 * time.Second}); err != nil {
        nn.storage = nil
        log.Println("ERROR NOTIFICATION.storage.Open:", err)
    } else { nn.storage = db }
    log.Println("Инициализировано хранилище сообщений. Path:", nn.storage.Path(), " Stats:", nn.storage.Stats())

    nn.messag_event = make(chan MESSAGE, SZ_EVENT_CHAN)     // буфферизированный канал событий-извещений для отправителя оповещений
    go func (){
        defer nn.recoveryNotification()
        for{                                // Ожидание событий и запуск процесса оповещения
            time.Sleep(time.Millisecond * time.Duration(10))
            if ev, ok := <- nn.messag_event; ok && ev.tm.Unix() > 10000 && ev.msg != "" {
                dubl := false
                if nn.storage != nil {					// сохранить для синхронизации с монитором, обеспечит надёжность отправки.
                    if err := nn.storage.Update(func(tx *bbolt.Tx) error {
                        if bucket, err := tx.CreateBucketIfNotExists([]byte(ev.tag)); err == nil && bucket != nil {
                            if tmptmu, tmpmsg := bucket.Cursor().Last(); tmptmu != nil && tmpmsg != nil && (ev.tm.UnixMilli() - int64(binary.BigEndian.Uint64(tmptmu))) < 10000 && bytes.Compare(tmpmsg, []byte(ev.msg)) == 0 {   // 10 sec последнее свежие сообщение сравнить на дублирование
                                dubl = true
                            } else {
                                return bucket.Put(ipc.Uint2Array(uint64(ev.tm.UnixMilli())), []byte(ev.msg))
                            }
                        } else if err != nil { return err }
                            return nil
                    }); err != nil {
                        log.Println("ERROR saveNotification:", err)
                        return
                    }
                }

                if !dubl {	// если это не дубль, то постораемся отправить за три попытки
                    go func() {
                        if err := nn.sendMessage(ev); err != nil {
                            time.Sleep(time.Second * time.Duration(20))
                            if err = nn.sendMessage(ev); err != nil {
                                time.Sleep(time.Second * time.Duration(40))
                                if err = nn.sendMessage(ev); err != nil {
                                    log.Println("ERROR FCM SendMessage:", err)	// МОЖНО продублировать в email !!!
                                }
                            }
                        }
                        if err := nn.send2Monitor(222,ev); err != nil {// отправить извещение-222 в Сервис Мониторинга
                            log.Println("ERROR Send2Monitor:", err)
                        }
                    }()
                }

            } else if !ok { break }
        } // for безусловный
    }()

    _minutes_ticker := time.NewTicker(time.Minute * 15)      // 15 минутный ТАЙМЕР чистки и синхронизации
    go func() {
        defer nn.recoveryNotification()
        for range _minutes_ticker.C {
            if nn.storage == nil {
                time.Sleep(time.Second * time.Duration(60))
                break
            }

            tmnow := time.Now()
            if err := nn.storage.Update(func(tx *bbolt.Tx) error {		// Чистка и синхронизация БД
                tx.ForEach(func(bkey []byte, bucket *bbolt.Bucket) error {	// Получим все корзины
                    if bucket != nil && bucket.Stats().KeyN > 0 {
                        bucket.ForEach(func(tmu []byte, msg []byte) error {
                            if tmu != nil && msg != nil {
                                if err := nn.send2Monitor(111, MESSAGE{tm:time.UnixMilli(int64(binary.BigEndian.Uint64(tmu))), tag:string(bkey), msg:string(msg)}); err == nil {
                                    log.Printf("-- SYNC: TM:%s  Tag:%s  Msg:%s", time.UnixMilli(int64(binary.BigEndian.Uint64(tmu))).Format("2006-01-02 15:04:05.000"), string(bkey), string(msg) )
                                    bucket.Delete(tmu)		// удалить отправленное оповещения !!!
                                } else {
                                    log.Println("ERROR SYNC send2Monitor.tmu:", time.UnixMilli(int64(binary.BigEndian.Uint64(tmu))).Format("2006-01-02 15:04:05.000"), len(msg), string(msg), err)
                                }
                            }
                            return nil
                        })

                        maxd := uint64(tmnow.Add(-1440 * time.Minute).UnixMilli())	// хранение 3 суток Duration(60*24=1440)   *3
                        c := bucket.Cursor()
                        for tmu, msg := c.Seek(ipc.Uint2Array(maxd)); tmu != nil; tmu, _ = c.Prev() {
                            if binary.BigEndian.Uint64(tmu) < maxd {
                                log.Printf("-- DELETE OLD: TM:%s  Tag:%s  Msg:%s", time.UnixMilli(int64(binary.BigEndian.Uint64(tmu))).Format("2006-01-02 15:04:05.000"), string(bkey), string(msg) )
                                bucket.Delete(tmu) // удалить старые оповещения !!!
                            }
                        }
                    } // if bucket
                    return nil
                })
                return nil
            }); err != nil {
                log.Println("ERROR Notification.storage.Clear:", err)
            }
        } // for C
    }()
}

//---------------------------------------------------------------------------

func (nn *NOTIFICATION) recoveryNotification() { // При сбоях в работе сервиса
    if recoveryMessage := recover(); recoveryMessage != nil {
        log.Println("recoveryNotification():", recoveryMessage, string(debug.Stack()), "\n****\n")
    }
}

//---------------------------------------------------------------------------

func (nn *NOTIFICATION) send2Monitor(tp byte, v MESSAGE) error {	// Отправка с подтверждением, для надёжности доставки оповещений.
    if monitor_addr == "" { return fmt.Errorf("не указан адрес получателя!") }
    if len(v.msg) > 200 {
        v.msg = v.msg[:200]		// Урезаем длинные сообщения !!!
        v.msg += `~`
    }
    log.Println("Notification.send2Monitor:", len(v.msg), v.msg)

    var bf [8]byte
    binary.BigEndian.PutUint64(bf[:], uint64(v.tm.UnixMilli()))		// время
    var data = []byte{tp}						// 1 байт - тип пакета
    data = append(data, bf[:]...)					// 8 байт - время
    data = append(data, []byte(v.tag)...)                               // добавим тэг
    data = append(data, ':')                                            // разделим :
    data = append(data, []byte(v.msg)...)                               // дополним сообщением
    data = append(data, 0)                                              // завершим 0
    return ipc.SendSHAEvent(monitor_addr, data)
}

//---------------------------------------------------------------------------

func (nn *NOTIFICATION) sendMessage(v MESSAGE) error {	// for mobile FCM-app
    priority := "normal"
    if len(v.tag) > 0 && v.tag[len(v.tag)-1] == '!' { priority = "high" }
    log.Println(" >>>>>>>>>>>>>>>> send2FCM():", priority, v.tm.Format("2006-01-02 15:04:05.000"), v.tag, v.msg )

    if len(v.msg) > 200 {
        v.msg = v.msg[:200]		// Урезаем длинные сообщения !!!
        v.msg += `~`
    }

    message := &messaging.Message{
        Android: &messaging.AndroidConfig{Priority: priority}, // Установка высокого приоритета [4, 5]
        Data: map[string]string{
            "tmu": fmt.Sprintf("%d",v.tm.UnixMilli()),
            "tag": v.tag,
            "msg": v.msg,
        },
        Topic: nn.secret.FCMTopic,
    }
    ctx := context.Background()
    app, err := firebase.NewApp(ctx, nil, nn.FCMOption)	// option.WithCredentialsFile("./host/data/"+srt.FCMFile)
    if err != nil {
        return fmt.Errorf("initializing send2FCM: %v", err)
    }

    client, err := app.Messaging(ctx)
    if err != nil {
        return fmt.Errorf("client send2FCM: %v", err)
    }
    response, err := client.Send(ctx, message)
    if err != nil {
        return fmt.Errorf("fatal send2FCM: %v", err)
    }
    log.Println("Successfully sent message:", response)
    return nil
}

//---------------------------------------------------------------------------

/* func (nn *NOTIFICATION) send2Telegram(v MESSAGE) error {		//         telegrammSend(&secret, message) -- ЗАБЛОКИРОВАЛИ :(
    if len(v.msg) < 3 { v.msg += "???" }
    qstr := `{"chat_id":"`+nn.secret.TgChatID+`","text":"`+v.msg+`"}`
    req, err := http.NewRequest( "POST", "https://api.telegram.org/bot"+nn.secret.TgToken+"/sendMessage", bytes.NewBufferString(qstr))
    if err == nil {
        req.ContentLength = int64(len(qstr))
        req.Header.Add("Content-Type", "application/json")
        req.Header.Add("Content-Length", fmt.Sprintf("%d", req.ContentLength))
        req.Header.Add("User-Agent", "SMonitor")

        client := &http.Client{}
        resp, err := client.Do(req)
        if err == nil {
            defer resp.Body.Close()

            if resp.StatusCode == 200 {
                if res, er := ioutil.ReadAll(resp.Body); er == nil {
                    log.Println("telegrammSend RES:", string(res))	// НАДО проверить результат
                    return nil
                } else { err = er }
            }
            err = fmt.Errorf("resp.StatusCode:%d", resp.StatusCode)
        }
        return err
    }
    return err
} */
//---------------------------------------------------------------------------
