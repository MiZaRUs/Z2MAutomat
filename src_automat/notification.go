/****************************************************************************
 *     Created  in  2025-2026  by  Oleg Shirokov   oleg@shirokov.online     *
 ****************************************************************************/

package main

import (
    "log"
    "fmt"
    "bytes"
    "time"
    "encoding/binary"
    "net/http"
    "io/ioutil"
    "go.etcd.io/bbolt"
    "../ipc"
)

//----------------------------------------
//Level   -  Bucket
//TMU     -  key
//msg     -  val
//---------------------------------------------------------------------------

func (s *service) sendNotification(level uint64, tm time.Time, msg string){
    if s.queue == nil || level < 1 || level > 99 || msg == "" { return }
    log.Println("sendNotification():", level, tm.Format("2006-01-02 15:04:05.000"), msg )
// level 1 - Авария (Протечка, взлом) 2 - прочее до 99
    if err := s.queue.Update(func(tx *bbolt.Tx) error {
        if bucket, err := tx.CreateBucketIfNotExists(ipc.Uint2Array(level)); err == nil && bucket != nil {
            return bucket.Put(ipc.Uint2Array(uint64(tm.UnixMilli())), []byte(msg))
        } else if err != nil { return err }
            return nil
    }); err != nil {
        log.Println("ERROR pushNotification:", err)
        return
    }
    s.messag_event <- level
}
//---------------------------------------------------------------------------

func (s *service) checkNotification(tmnow time.Time, lvl uint64) {
    if s.queue == nil { return }
//    log.Println("checkNotification ", lvl)

    if lvl > 0 && lvl < 100 {	// level 1..99 - свежие сообщение
        var tmp_tmu []byte
        var tmp_msg []byte

        if err := s.queue.View(func(tx *bbolt.Tx) error {
            level := tx.Bucket(ipc.Uint2Array(lvl))		// Получим корзину (level)
            if level != nil {
                if tmu, msg := level.Cursor().Last(); tmu != nil && msg != nil  && (tmnow.UnixMilli() - int64(binary.BigEndian.Uint64(tmu))) < 2000 {	// 2 sec последнее свежие сообщение
                    tmp_tmu = tmu
                    tmp_msg = msg
                    return nil
                }
            }
            return fmt.Errorf("очень плохо, нет такого сообщения!")	// ПРОБЛЕМА !!!
        }); err != nil {
            log.Println("ERROR QueueDB.Notification-t1:", err)
        }
        if tmp_tmu == nil || tmp_msg == nil { return }	// Обязательно должно быть свежее сообщение !!!


        if err := s.queue.View(func(tx *bbolt.Tx) error {		// Поиск дубля в кэше
            if cashe := tx.Bucket(ipc.Uint2Array(0)); cashe != nil {	// Получим корзину (0) - отправленные, последнее свежие сообщение
                maxd := uint64(tmnow.Add(-10*time.Minute).UnixMilli())	// 10 минут !!!!!!!!!!!!!!!
                cashe.ForEach(func(tmu []byte, msg []byte) error {
                    if tmu != nil && msg != nil && binary.BigEndian.Uint64(tmu) > maxd && bytes.Compare(tmp_msg, msg) == 0 {	// сравнить сообщение с кэшем отправки !!! не устарело
                        tmp_tmu = nil
                        tmp_msg = nil
                    }
                    return nil
                })
            }
            return nil
        }); err != nil { log.Println("ERROR QueueDB.Notification-t2:", err) }


        if tmp_tmu != nil && tmp_msg != nil {
            if err := telegramSend(&s.secret, string(tmp_msg)); err == nil {
                if err := s.queue.Update(func(tx *bbolt.Tx) error {
                    if cashe, err := tx.CreateBucketIfNotExists(ipc.Uint2Array(0)); err == nil && cashe != nil {	// Получим корзину (кэш) - отправленные
                        return cashe.Put(tmp_tmu, tmp_msg)
                    } else {
                        return err
                    }
                }); err != nil {
                    log.Println("ERROR QueueDB.Notification-t3:", err)
                    return
                }
            } else {
                log.Println("ERROR QueueDB.Notification.telegrammSend:", err)
            }
        }

    } // level 1..99


    if err := s.queue.Update(func(tx *bbolt.Tx) error {		// Чистка БД
        tx.ForEach(func(lvl []byte, bucket *bbolt.Bucket) error { // Получим все корзины (уровни оповещений)

// НАДО поиск неотправленных до очистки кэша !!!

            level := binary.BigEndian.Uint64(lvl)
//            if bucket != nil { log.Println("QueueDB.Clear-X:", level, bucket.Stats().KeyN) }
            if bucket != nil && bucket.Stats().KeyN > 0 {
                ddel := time.Duration(60)			// временное хранение	60*25 час
                if level == 0 { ddel = 10 }			// кэш 5-15 минут	!!!!!!!!!!!!!!!!!!!!!!!
                if level == 1 { ddel = 60*3 }			// длительное хранение 3 суток Duration(60*24*3)
                maxd := uint64(tmnow.Add(-1*ddel * time.Minute).UnixMilli())
                c := bucket.Cursor()
                for tmu, msg := c.Seek(ipc.Uint2Array(maxd)); tmu != nil; tmu, _ = c.Prev() {
                    if binary.BigEndian.Uint64(tmu) < maxd {
                        log.Printf("-- DELETE: lvl:%d  TM:%s  Msg:%s", level, time.UnixMilli(int64(binary.BigEndian.Uint64(tmu))).Format("2006-01-02 15:04:05.000"), string(msg) )
                        bucket.Delete(tmu) // удалить старые оповещения !!!
                    }
                }
            }
            return nil
        })
        return nil
    }); err != nil {
        log.Println("ERROR QueueDB.Notification-X:", err)
    }
}

//---------------------------------------------------------------------------

func telegramSend(srt *SECRET, msg string) error {		//         telegrammSend(&secret, message)
    if len(msg) < 3 { msg += "???" }
    qstr := `{"chat_id":"`+srt.TgChatID+`","text":"`+msg+`"}`
    req, err := http.NewRequest( "POST", "https://api.telegram.org/bot"+srt.TgToken+"/sendMessage", bytes.NewBufferString(qstr))
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
}

//---------------------------------------------------------------------------
