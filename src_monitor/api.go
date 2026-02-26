/****************************************************************************
 *     Created  in  2025-2026  by  Oleg Shirokov   oleg@shirokov.online     *
 ****************************************************************************/

package main

import (
    "log"
    "fmt"
    "time"
//    "strings"
//    "bytes"
    "math"
    "encoding/binary"
    "go.etcd.io/bbolt"
    "../ipc"
)

//---------------------------------------------------------------------------

func (sm *SM) getMetricsJSON(uid, sens string, tmin, tmax uint64)(jstr string) {
//    log.Println("getMetricsJSON:", uid, sens, tmin, tmax)
    dtmu := time.Now().UnixMilli() - 100000000		// ограничитель в милисек. 86400 сек = сутки
    jstr = "["
    if err := sm.mdb.View(func(tx *bbolt.Tx) error {
        if device := tx.Bucket([]byte(uid)); device != nil {
            if sensor := device.Bucket([]byte(sens)); sensor != nil {
                c := sensor.Cursor()
// варианты запроса в зависимости от значений tmin и tmax !!! ( несколько последних записей)
                if dtmu > int64(tmax-tmin) && (tmax > 1000 && tmin > 1000){		// с ограничением по длине
                    for k, v := c.Seek(ipc.Uint2Array(tmin)); k != nil; k, v = c.Next() { // for k, v := c.Last(); k != nil; k, v = c.Prev() // for k, v := c.First(); k != nil; k, v = c.Next() 
                        if  binary.BigEndian.Uint64(k) <= tmax {
                            jstr += fmt.Sprintf(`{"tmu":%d,"val":%f},`, binary.BigEndian.Uint64(k), math.Float64frombits(binary.BigEndian.Uint64(v)))
                        } else { break }
                    }
                } else if tmax == 0 && dtmu > (time.Now().UnixMilli() - int64(tmin)) {	// от заданного до конца с ограничением по длине
                    for k, v := c.Seek(ipc.Uint2Array(tmin)); k != nil; k, v = c.Next() {
                        jstr += fmt.Sprintf(`{"tmu":%d,"val":%f},`, binary.BigEndian.Uint64(k), math.Float64frombits(binary.BigEndian.Uint64(v)))
                    }
                } else {
                    k, v := c.Last()                          // последнее значение
                    jstr += fmt.Sprintf(`{"tmu":%d,"val":%f},`, binary.BigEndian.Uint64(k), math.Float64frombits(binary.BigEndian.Uint64(v)))
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
