/****************************************************************************
 *     Created  in  2025-2026  by  Oleg Shirokov   oleg@shirokov.online     *
 ****************************************************************************/

package main

import (
    "log"
    "fmt"
    "time"
    "encoding/json"
//    "encoding/binary"
//    "go.etcd.io/bbolt"
//    "ipc"
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

type SECRET struct {    // данные для подключения к сервисам оповещения (FCM, telegram, jabber, mail ... )
    FCMFile   string	`json:"fcm_file"`
    FCMTopic  string	`json:"fcm_topic"`
//    MailBox  string	`json:"mailbox"`
//    MailPSWD string	`json:"mailpswd"`
//    TgToken   string	`json:"tg_token"`
//    TgChatID  string	`json:"tg_chatid"`
}

type NOTIFICATION struct {
    FCMOption option.ClientOption
    secret    SECRET			// данные для подключения к внешним сервисам
//    storage   *bbolt.DB			// хранилище всех оповещений
    messag_event  chan MESSAGE		// Событие-извещение для отправителя сообщений
}

//----------------------------------------
//   -  Bucket
//TMU     -  key
//msg     -  val
//---------------------------------------------------------------------------

func (nn *NOTIFICATION) Send(tm time.Time, tag, msg string){	// Оповещения		Информация, Внимыние!, АВАРИЯ!
    if nn.messag_event != nil { nn.messag_event <- MESSAGE{tm:tm, tag:tag, msg:msg} }	// упорядочить
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

    nn.messag_event = make(chan MESSAGE, 7)                // канал событий-извещений для отправителя оповещений

    go func (){
        defer nn.recoveryNotification()
        for{                                // Ожидание событий и запуск процесса оповещений
            time.Sleep(time.Millisecond * time.Duration(10))
            if ev, ok := <- nn.messag_event; ok && ev.tm.Unix() > 10000 && ev.msg != "" {
                go func() {

                    if err := nn.fcmSend(ev); err != nil {
                        log.Println("ERROR FCM SendMessage:", err)
                        log.Println(" ++++++ Send2Mail +++++++", ev)	// НАДО при ошибке FCM отправлять на почту
                    }

                    if ev.tag == "АВАРИЯ!" {				// НАДО тег "АВАРИЯ!" продублировать на почту
                        log.Println(" ++++++ Send2Mail +++++++", ev)
                    }
                }()
            } else if !ok { break }
        } // for безусловный
    }()
}

//---------------------------------------------------------------------------

func (nn *NOTIFICATION) recoveryNotification() { // При сбоях в работе сервиса
    if recoveryMessage := recover(); recoveryMessage != nil {
        log.Println("recoveryNotification():", recoveryMessage, string(debug.Stack()), "\n****\n")
    }
}

//---------------------------------------------------------------------------

func (nn *NOTIFICATION) fcmSend(v MESSAGE) error {	// for mobile FCM-app
    priority := "normal"
    if len(v.tag) > 0 && v.tag[len(v.tag)-1] == '!' { priority = "high" }
    log.Println(" >>>>>>>>>>>>>>>> fcmSend():", priority, v.tm.Format("2006-01-02 15:04:05.000"), v.tag, v.msg )

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
        return fmt.Errorf("initializing fcmSend: %v", err)
    }

    client, err := app.Messaging(ctx)
    if err != nil {
        return fmt.Errorf("client fcmSend: %v", err)
    }
    response, err := client.Send(ctx, message)
    if err != nil {
        return fmt.Errorf("fatal fcmSend: %v", err)
    }
    log.Println("Successfully sent message:", response)
    return nil
}

//---------------------------------------------------------------------------
