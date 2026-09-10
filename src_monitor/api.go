/****************************************************************************
 *      Created  in  2025-2026  by  Oleg Shirokov   olgshir@gmail.com       *
 ****************************************************************************/

package main

import (
    "log"
    "fmt"
    "time"
    "strings"
//    "bytes"
//    "math"
    "encoding/binary"
    "go.etcd.io/bbolt"
    "ipc"
)

// НАДО отправить конфиг в приложение (uid : name [:описание?])

//---------------------------------------------------------------------------

// список извещений за определённый провежуток времени (для синхронизации)
func (sm *SM) getNotificationsJSON(tmin, tmax uint64)(jstr string) {
//    log.Println("getEventMessagesJSON:", tmin, tmax)
    dtmu := time.Now().UnixMilli() - 100000000		// ограничитель в милисек. 86400 сек = сутки
    jstr = "["
    if err := sm.mdb.View(func(tx *bbolt.Tx) error {
        if evn := tx.Bucket([]byte("*EVENTS*")); evn != nil {		// События-извещения
            if msg := evn.Bucket([]byte("*Messages*")); msg != nil {
                c := msg.Cursor()
// варианты запроса в зависимости от значений tmin и tmax !!! ( несколько последних записей)
                if dtmu > int64(tmax-tmin) && (tmax > 1000 && tmin > 1000){		// с ограничением по длине
                    for k, v := c.Seek(ipc.Uint2Array(tmin)); k != nil; k, v = c.Next() { // for k, v := c.Last(); k != nil; k, v = c.Prev() // for k, v := c.First(); k != nil; k, v = c.Next() 
                        if  binary.BigEndian.Uint64(k) <= tmax {
                            if vx := strings.SplitN(string(v), ":", 2); len(vx) == 2 && vx[0] != "" && vx[1] != "" {
                                jstr += fmt.Sprintf(`{"tmu":%d,"tag":"%s","msg":"%s"},`, binary.BigEndian.Uint64(k), vx[0], vx[1])
                            }
                        } else { break }
                    }
                } else if tmax == 0 && dtmu > (time.Now().UnixMilli() - int64(tmin)) {	// от заданного до конца с ограничением по длине
                    for k, v := c.Seek(ipc.Uint2Array(tmin)); k != nil; k, v = c.Next() {
                        if vx := strings.SplitN(string(v), ":", 2); len(vx) == 2 && vx[0] != "" && vx[1] != "" {
                            jstr += fmt.Sprintf(`{"tmu":%d,"tag":"%s","msg":"%s"},`, binary.BigEndian.Uint64(k), vx[0], vx[1])
                        }
                    }
                } else {
                    k, v := c.Last()                          // последнее значение
                    if vx := strings.SplitN(string(v), ":", 2); len(vx) == 2 && vx[0] != "" && vx[1] != "" {
                        jstr += fmt.Sprintf(`{"tmu":%d,"tag":"%s","msg":"%s"},`, binary.BigEndian.Uint64(k), vx[0], vx[1])
                    }
                }
            }
        }
        return nil
    }); err != nil {
        log.Println("ERROR sm.mdb.View:", err)
    }
    if len(jstr) > 17 {
        jstr = jstr[:len(jstr)-1]         // заменим последнюю запятую
        jstr += "]"
        return jstr
    }
    return "{}"
}

//---------------------------------------------------------------------------
//    if uid == "CONF" && sens == "" && tmin < 1000 && tmax < 1000 {
//        log.Println("MDB * список всех устройств (КОНФИГ)")
//        return "{}"
//    }

// список метрик
func (sm *SM) getMetricsJSON(duid, sens string, tmin, tmax uint64)(jstr string) {
    log.Println("getMetricsJSON:", duid, sens, tmin, tmax)
    if duid == "*EVENTS*" || sens == "*Messages*" { return "{}" }	// исключить запрос сообщений: []byte("*EVENTS*") >> []byte("*Messages*") ???

    if err := sm.mdb.View(func(tx *bbolt.Tx) error {

    	if duid != "" && sens != "" {	// список метрик по указанному устройству-сенсору за определённый провежуток времени (для графиков)
            if device := tx.Bucket([]byte(duid)); device != nil {
//                log.Println("MDB.Bucket.Device.Size:", duid, device.Stats().KeyN )
                if sensor := device.Bucket([]byte(sens)); sensor != nil {
                    if str := getSensorValueToTimes(sensor.Cursor(), tmin, tmax, 1000); str != "" {
                        jstr = fmt.Sprintf(`{"duid":"%s","sens":"%s","arr":[%s]},`, duid, sens, str )
//                        log.Println("MDB.Bucket.D&S.cmp:", duid, sens, str)
                    }
                } else { log.Println("ERROR MDB.Bucket.Sensor:", duid, sens ) }
            }

        } else if duid != "" && sens == "" {		// Получить одно устройство по ключу и все сенсоры из
            if device := tx.Bucket([]byte(duid)); device != nil {
                device.ForEach(func(ns []byte, _ []byte) error {    // Перебор корзин (сенсор) в корзине (устойство)
                    str := ""
                    if sensor := device.Bucket(ns); sensor != nil {	 // Получим корзину (сенсор)
                        str = getSensorValueToTimes(sensor.Cursor(), tmin, tmax, 100)
//                        log.Println("MDB.Bucket.D&S.uid:", duid, string(ns), str)
                    }// else { log.Println("ERROR MDB.Bucket.Sensor:", duid, string(ns)) }
                    if str != "" {
                        jstr += fmt.Sprintf(`{"duid":"%s","sens":"%s","arr":[%s]},`, duid, string(ns), str )
                    }
                    return nil
                })
            }

        } else if duid == "" && sens != "" {		// Получить все устройства и один сенсор по ключу
            tx.ForEach(func(nd []byte, device *bbolt.Bucket) error { // Получим все корзины (устойство)
                if device != nil && string(nd) != "*EVENTS*" {	// исключить корзину сообщений: []byte("*EVENTS*") >> []byte("*Messages*") !!!
//                    log.Println("MDB.Bucket.Device.Size:", string(nd), sens, device.Stats().KeyN )
                    str := ""
                    if sensor := device.Bucket([]byte(sens)); sensor != nil {
                        str = getSensorValueToTimes(sensor.Cursor(), tmin, tmax, 40)
//                        log.Println("MDB.Bucket.D&S.sens:", string(nd), sens, str)
                    } // noerror - не вовсех устройствах есть необходимый сенсор
                    if str != "" {
                        jstr += fmt.Sprintf(`{"duid":"%s","sens":"%s","arr":[%s]},`, string(nd), sens, str )
                    }
                }
                return nil
            })

	} else {	// Получить все устройства и все сенсоры из
            tx.ForEach(func(nd []byte, device *bbolt.Bucket) error { // Получим все корзины (устойство)
                if device != nil && string(nd) != "*EVENTS*" {	// исключить корзину сообщений: []byte("*EVENTS*") >> []byte("*Messages*") !!!
//                    log.Println("MDB.Bucket.Device.Size:", string(nd), device.Stats().KeyN )
                    device.ForEach(func(ns []byte, _ []byte) error {    // Перебор корзин (сенсор) в корзине (устойство)
                        str := ""
                        if sensor := device.Bucket(ns); sensor != nil {	 // Получим корзину (сенсор)
                            str = getSensorValueToTimes(sensor.Cursor(), tmin, tmax, 10)
//                            log.Println("MDB.Bucket.D&S.all:", string(nd), string(ns), str)
                        }// else { log.Println("ERROR MDB.Bucket.Sensor:", string(nd), string(ns)) }
                        if str != "" {
                            jstr += fmt.Sprintf(`{"duid":"%s","sens":"%s","arr":[%s]},`, string(nd), string(ns), str)
                        }
                        return nil
                    })
                }
                return nil
            })

        }

        return nil
    }); err != nil {
        log.Println("ERROR sm.mdb.View:", err)
    }

    if len(jstr) > 17 {
        jstr = "["+jstr[:len(jstr)-1]+"]"         // заменим последнюю запятую
//        log.Println("RES:", jstr)
        return jstr
    }
    return "{}"
}

//---------------------------------------------------------------------------

// варианты запроса в зависимости от значений tmin и tmax !!!
func getSensorValueToTimes(c *bbolt.Cursor, tmin, tmax uint64, ix int)(jstr string) {
    if tmin > 1000 && tmax > 1000 && int64(tmax-tmin) < 200000000 {	// с ограничением по количеству и времени в милисек. 86 400 сек = сутки
//        log.Println("MDB * с", tmin, " по", tmax)
        for k, v := c.Seek(ipc.Uint2Array(tmin)); k != nil; k, v = c.Next() {
            if binary.BigEndian.Uint64(k) <= tmax && ix > 0 { // ограничитель по количеству
                ix -= 1
                jstr += fmt.Sprintf(`{"tmu":%d,"val":%d},`, binary.BigEndian.Uint64(k), binary.BigEndian.Uint64(v))
            } else { break }
        }

    } else if tmin == 0 && tmax > 1000 {	// log.Println("MDB * за указанное число (последнее что было до него)")
	k, v := c.Seek(ipc.Uint2Array(tmax))	// первый элемент, который больше или равен цели (>= tmax)
        if k == nil {
	    k, v = c.Last()
	} else if binary.BigEndian.Uint64(k) >= tmax {
	    k, v = c.Prev()	// чтобы получить первое значение строго ПЕРЕД указанной датой.
	}
	if k != nil {
            jstr += fmt.Sprintf(`{"tmu":%d,"val":%d},`, binary.BigEndian.Uint64(k), binary.BigEndian.Uint64(v))
        }

    } else if tmin > 1000 && tmax == 0 {	// log.Println("MDB * за указанное число (первое что есть после него)")
	k, v := c.Seek(ipc.Uint2Array(tmin))	// первый элемент, который больше или равен цели (>= tmax)
	if k != nil && binary.BigEndian.Uint64(k) == tmin {
	    if kx, vx := c.Next(); kx != nil {
                k = kx
                v = vx
            }
            jstr += fmt.Sprintf(`{"tmu":%d,"val":%d},`, binary.BigEndian.Uint64(k), binary.BigEndian.Uint64(v))
	}

    } else { // tmin < 1000 && tmax < 1000 последняя запись
        if k, v := c.Last(); k != nil {
            jstr += fmt.Sprintf(`{"tmu":%d,"val":%d},`, binary.BigEndian.Uint64(k), binary.BigEndian.Uint64(v))
        }
    }
//                log.Printf(`{"tmu":%d,"val":%d :%X},`, binary.BigEndian.Uint64(k), binary.BigEndian.Uint64(v), v)
    if len(jstr) > 17 { jstr = jstr[:len(jstr)-1] }         // удалим последнюю запятую
    return jstr
}

// jstr += fmt.Sprintf(`{"duid":"%s","sens":"%s","tmu":%d,"val":%f},`, string(nd), string(ns), binary.BigEndian.Uint64(k), math.Float64frombits(binary.BigEndian.Uint64(v)))
// jstr += fmt.Sprintf(`{"tmu":%d,"val":%f},`, binary.BigEndian.Uint64(k), math.Float64frombits(binary.BigEndian.Uint64(v)))
//---------------------------------------------------------------------------

// for k, v := c.Last(); k != nil; k, v = c.Prev() // for k, v := c.First(); k != nil; k, v = c.Next() 
// ( несколько последних записей)
//    dtmu := time.Now().UnixMilli() - 100000000		// ограничитель в милисек. 86400 сек = сутки
//     && dtmu > (time.Now().UnixMilli() - int64(tmin)) {	// от заданного до конца с ограничением по длине
//        for k, v := c.Seek(ipc.Uint2Array(tmin)); k != nil; k, v = c.Next() {
//            jstr += fmt.Sprintf(`{"tmu":%d,"val":%f},`, binary.BigEndian.Uint64(k), math.Float64frombits(binary.BigEndian.Uint64(v)))
//        }
// Целевое время (например, 7 сентября 2026)
//    targetTime := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
//    targetBytes := timeToBytes(targetTime.UnixMilli())

//получить первое значение перед указанной датой

/*func GetValueBeforeDate(db *bbolt.DB, bucketName []byte, targetTime time.Time) ([]byte, error) {
    // 1. Переводим целевое время в UnixMilli и кодируем в BigEndian байты
    targetBytes := make([]byte, 8)
    binary.BigEndian.PutUint64(targetBytes, uint64(targetTime.UnixMilli()))
    var value []byte
    err := db.View(func(tx *bbolt.Tx) error {
	b := tx.Bucket(bucketName)
	if b == nil {
	    return bbolt.ErrBucketNotFound
	}

	c := b.Cursor()
	k, v := c.Seek(targetBytes)	// первый элемент, который больше или равен цели (>= targetBytes)

	// 3. Сдвигаемся на шаг назад
	if k == nil {
	    // Если Seek вернул nil, значит targetTime больше ВСЕХ ключей в базе.
	    // Самое последнее значение в базе и есть искомое "перед этой датой".
	    k, v = c.Last()

	} else {	//	} else if bytes.Compare(k, targetBytes) >= 0 {

//Поэтому, чтобы гарантированно получить значение строго перед (<) указанной датой, нужно сделать один шаг назад с помощью c.Prev().
	    // Если мы нашли ключ, который >= targetBytes, нам нужно сделать шаг назад,
	    // чтобы получить элемент, который строго МЕНЬШЕ (<) targetBytes.
	    // Это сработает и при точном совпадении, и если Seek ушел вперед.

	    // Если найденный ключ больше или равен целевому, делаем шаг назад,
	    // чтобы получить первое значение строго ПЕРЕД указанной датой.

	    k, v = c.Prev()
	}


// 2. Если нужно строгое соответствие (>) и мы попали ровно на targetBytes,
// Параметр strict определяет условие: 
// true  -> строго больше ( > target )
// false -> больше или равно ( >= target )
// то делаем один шаг вперед.
//	if strict && k != nil && bytes.Equal(k, targetBytes) {
//	    k, v = c.Next()
//	}


	// Если после всех сдвигов k != nil, значит мы нашли нужную запись
	if k != nil {
	    // Копируем байты, так как данные bbolt доступны только внутри транзакции
	    value = make([]byte, len(v))
	    copy(value, v)
	}

	return nil
    })

    return value, err
}*/
//-----------


