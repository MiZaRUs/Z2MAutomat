/****************************************************************************
 *     Created  in  2025-2026  by  Oleg Shirokov   oleg@shirokov.online     *
 ****************************************************************************/

package main

import (
    "log"
    "time"
    "strings"
    "bytes"
    "encoding/binary"
    "go.etcd.io/bbolt"
    "../ipc"
)

//---------------------------------------------------------------------------
//UID     -  Bucket
//Sensor  -  Bucket
//TMU     -  key
//Value   -  val
//---------------------------------------------------------------------------

func (sm *SM) saveMetrics(data []byte) {
    for {
        if len(data) < 36 { break }
        it := 33
        if i := bytes.IndexByte(data[it:], 0); i == -1 {	// ищем завершающий '0'
            break	// не найдено
        } else {
            it += i
        }
        if lid := strings.Split(string(data[16:it]), `:`); len(lid)==2 && len(lid[0])==18 && len(lid[1])>3 {
            if err := sm.mdb.Update(func(tx *bbolt.Tx) error {
                if device, err := tx.CreateBucketIfNotExists([]byte(lid[0])); err == nil && device != nil {
                    if sensor, err := device.CreateBucketIfNotExists([]byte(lid[1])); err == nil && sensor != nil {
                        _, v := sensor.Cursor().Last()				// последнее значение
                        if bytes.Compare(v, data[8:16]) == 0 { return nil }	// проверить наличее такого же значения в последней записи
//                        log.Printf(" * Metric: %s : %s -- %X : %X", lid[0], lid[1], data[0:8], data[8:16])
                        return sensor.Put(data[0:8], data[8:16])
                    } else if err != nil { return err }
                } else if err != nil { return err }
                    return nil
            }); err != nil {
                log.Println("ERROR IPC.saveMetric:", err)
            }
        }
        data = data[it+1:]
    }
}

//---------------------------------------------------------------------------

func (sm *SM) checkMDBStatus() {        // Мониторинг состояния хранилища
    log.Printf("Стартуем процесс мониторинга.")
    defer log.Printf("ERROR Завершён процесс мониторинга!")

    tmSafeKeeping := time.Now()		// чистка БД
    for {
        time.Sleep(time.Second * time.Duration(60))     // Раз в минуту

        if int(time.Now().Sub(tmSafeKeeping).Seconds()) > 3600 { // 3600 - чистка БД каждый час
            tmSafeKeeping = time.Now()
            if err := sm.mdb.Update(func(tx *bbolt.Tx) error {
                tx.ForEach(func(nd []byte, device *bbolt.Bucket) error { // Получим все корзины (устойство)
                if device != nil {
                    log.Println("MDB.Bucket.Device.Size:", string(nd), device.Stats().KeyN )
//                    if device.Stats().KeyN == 0 { tx.DeleteBucket(nd) }  // НАДО удалять пустые корзины !!!

                    device.ForEach(func(ns []byte, _ []byte) error {	// Перебор корзин (сенсор) в корзине (устойство)
                        ddel := time.Duration(24*3)			// постоянное хранение 3 суток
                        if string(ns) == "illuminance" { ddel = 25 }	// временное хранение  {illuminance}
                        maxd := ipc.Int2Array(uint64(tmSafeKeeping.Add(-1*ddel * time.Hour).UnixMilli()))
                        log.Println("MDB.Bucket.D&S:", time.UnixMilli(int64(binary.BigEndian.Uint64(maxd))).Format("2006-01-02 15:04:05.000"), string(nd), string(ns))

                        sensor := device.Bucket(ns)			// Получим корзину (сенсор)
                        c := sensor.Cursor()
                        for k, _ := c.Seek(maxd); k != nil; k, _ = c.Prev() {
                            if binary.BigEndian.Uint64(k) < binary.BigEndian.Uint64(maxd) {
//                                log.Printf("DELETE: %s : %s : %s", string(nd), string(ns), time.UnixMilli(int64(binary.BigEndian.Uint64(k))).Format("2006-01-02 15:04:05.000") )
                                sensor.Delete(k) // удалить !!!
                            }
                        }
                        return nil
                    })
                }
                    return nil
                })
                return nil
            }); err != nil {
                log.Println("ERROR sm.mdb.Update:", err)
            }
            log.Println("-----------")				// DEBUG !!!
        }
    } // безусловный for
}

//---------------------------------------------------------------------------
