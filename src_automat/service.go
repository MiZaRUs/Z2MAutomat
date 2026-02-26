/****************************************************************************
 *     Created  in  2025-2026  by  Oleg Shirokov   oleg@shirokov.online     *
 ****************************************************************************/

package main

import (
    "log"
    "fmt"
    "time"
    "strings"
    "sync"
    "math/rand"
    "runtime/debug"
    "os"
    "go.etcd.io/bbolt"
    MQTT "github.com/eclipse/paho.mqtt.golang"
)

//----------------------------------------

const (
    Z2M = "zigbee2mqtt/"	// префикс топика
    monitor_addr = "localhost:10101"
)

//----------------------------------------

type service struct {
//    mut     sync.Mutex
    mut      sync.RWMutex
    queue   *bbolt.DB			// Очередь важных сообщений

    device_index  map[string]*ZBDev	// Конфигурации устройств
    timer_index   map[string]*time.Timer
    sensor_event  chan *ZBDev		// Событие от mqtt подписки
    client MQTT.Client

    debugLog  bool

// flags
    automatic bool              // НАДО обозначить индикацию !!!

    obj_active_tm time.Time	// время последней активности объекта
}

//---------------------------------------------------------------------------

func (s *service) activatedZ2MSubscribe(topic string, msg []byte) {	// zigbee2mqtt (Z2M)   -- горутина !!!
    if s.debugLog { log.Println(" +++", topic, string(msg) ) }
    if x := strings.Split(topic, "/"); len(x)==2 && len(x[1]) == 18 && len(msg) > 7 {
        if sev := s.updateZ2MDevice(x[1], msg); sev != nil && sev.uid != "" && sev.Name != "" {	// Обновить состояние устройства и если надо - создать событие
            s.sensor_event <- sev
        }
    }else{
        log.Println("ERROR ZBMSG:", topic," : ", string(msg) )
    }
}

//---------------------------------------------------------------------------

func (s *service) publish(topic string, qos byte, retained bool, msg string) {
    log.Println("MQTT.Publish:", topic, qos, retained, msg)
    token := s.client.Publish(topic, qos, retained, msg)
    if token.Wait() && token.Error() != nil {
        log.Println("ERROR MQTTPublish:",token.Error())
    }
    token.Wait()
}

func (s *service) subscribe(topic string, qos byte) error {
    token := s.client.Subscribe(topic, qos, nil)
    if token.Wait() && token.Error() != nil {
        return fmt.Errorf("MQTT Subscribe: %s",token.Error())
    }
    token.Wait()
    return nil
}

// -------------------------------------------------------------------------

// * функции обратного вызова
func (s *service) connectHandler(client MQTT.Client) {				// MQTT.OnConnectHandler
    log.Printf("MQTT.OnConnectHandler: ConnectMQTTClient Ok.")
}

func (s *service) connectLostHandler(client MQTT.Client, err error) {		// MQTT.ConnectionLostHandler
    log.Printf("MQTT.ConnectionLostHandler: %v", err)
}

func (s *service) messageHandler(client MQTT.Client, msg MQTT.Message) {	// MQTT.MessageHandler
    if strings.HasPrefix(msg.Topic(), Z2M) {				// разбор по признакам для дальнейшей обработки
        go s.activatedZ2MSubscribe(msg.Topic(), msg.Payload())
    }else{
        log.Println("MQTT.МessageHandler:", msg.Topic()," : ", msg.Qos()," : ", msg.Retained()," : ", msg.MessageID()," : ", string(msg.Payload()) )
    }
//    log.Println("messagePubHandler:", msg.Topic()," : ", msg.Qos()," : ", msg.Retained()," : ", msg.MessageID() )
}

//---------------------------------------------------------------------------

// * Инициализация сервиса и подключение к MQTT
func createService()(s service) {

    s.debugLog = false

    if s.debugLog {
        if err := os.Mkdir("./logs", 0777); err != nil && !os.IsExist(err) {	// os.MkdirAll("test/subdir",
            log.Println("ERROR FATAL", err)
            return
        }

        filemqttDebug, er1 := os.OpenFile("./logs/mqttDebug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
        if er1 != nil {
            log.Println("ERROR OpenFile:", er1)
        } else {
            log.Println("DebugLog: ON")
            MQTT.ERROR = log.New(filemqttDebug, "[ERROR] ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
            MQTT.CRITICAL = log.New(filemqttDebug, "[CRIT] ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
            MQTT.WARN = log.New(filemqttDebug, "[WARN]  ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
//            MQTT.DEBUG = log.New(filemqttDebug, "[DEBUG] ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
        }
    }


    s.timer_index  = make(map[string]*time.Timer)
    s.sensor_event = make(chan *ZBDev)	// События от устройств

    s.device_index = make(map[string]*ZBDev)	// Хранилище устройств
    loadDevicesConfig("dev.json", s.device_index)	// конфигурации устройств
    log.Println("Initialization.ZBDev:", len(s.device_index))

// * Инициализация клиентского подключения.
    opts := MQTT.NewClientOptions()
    opts.AddBroker("tcp://localhost:1883")		// broker   - адрес сервера, tcp://IP:PORT. Default: tcp://localhost:1883
    opts.SetUsername("")				// user     - Имя пользователя.
    opts.SetPassword("")				// password - Пароль.
    gen_id := fmt.Sprintf("ClMQTT:%v", uint32(rand.Uint32()))
    opts.SetClientID(gen_id)				// clientid    - Уникальный в пределах брокера id клиента. Если не указан то будет сгенерирован на основе кода сервиса.
    //cl.opts.SetProtocolVersion(3)
    opts.SetCleanSession(false)				// cleansess   - Использовать чистую сессию при подключении брокера, игнорируя флаг Retain. Default: false
    opts.SetKeepAlive(time.Duration(5) * time.Second)	// keepalive   - поддержка соединения,в секундах. Default: 5
    opts.SetPingTimeout(time.Duration(10) * time.Second)// pingtimeout - таймаут пинга,в секундах. Default: 10
    opts.SetDefaultPublishHandler(s.messageHandler)
    opts.OnConnect = s.connectHandler
    opts.OnConnectionLost = s.connectLostHandler
    opts.SetStore(MQTT.NewFileStore(":memory:"))	// store       - Store Directory. Default: :memory:

    log.Println("ConnectMQTTClient: Создаем клиента. ID:", gen_id)
    s.client = MQTT.NewClient(opts)
    log.Println("ConnectMQTTClient: Подключаем клиента к брокеру...")
    token := s.client.Connect()
    if token.Wait() && token.Error() != nil {
	log.Printf("ConnectMQTTClient %s", token.Error())
	s.client = nil
	return
    }
    return s
}

//---------------------------------------------------------------------------

func (s *service) recoveryService() { // При сбоях в работе сервиса
    if recoveryMessage := recover(); recoveryMessage != nil {
    	log.Println("Session.ClientSubscribeSession.Recovery:", recoveryMessage, string(debug.Stack()), "\n****\n")
    } else {
    	log.Println("Session.ClientSubscribeSession: END return.\nRESTART", "\n****\n")
    }

    if s.client != nil {
        s.client.Disconnect(250)
        log.Println("ConnectMQTTClient.client Disconnect MQTT Client!")
    }
    s.queue.Close()
    log.Println("WARNING Работа сервиса прекращена!\n\n\n")
}

//---------------------------------------------------------------------------

//===========================================================================
func main() {
    log.SetFlags(log.Ldate | log.Ltime)
    defer log.Println("WARNING Работа сервиса прекращена!\n\n\n")

    err := os.MkdirAll("./host/data", 0777)
    if err != nil && !os.IsExist(err) {
        log.Println("FATAL_ERROR MkDir data:", err)
        return
    }

    srv := createService()
    if srv.client == nil || srv.sensor_event == nil || srv.device_index == nil || len(srv.device_index) < 1 {
        return
    }
    defer srv.recoveryService()
//-------------------------------------
    log.Println("Создаём хранилище очереди сообщений")
    if srv.queue, err = bbolt.Open("./host/data/queue.db", 0600, &bbolt.Options{Timeout: 2 * time.Second}); err != nil {
        log.Println("FATAL_ERROR CreateQueueDB.Open:", err)
        return
    }
    log.Println("  QueueDB Path:", srv.queue.Path(), " Stats:", srv.queue.Stats())


// -- установить начальное состояние !
// srv.executeSetDefault()
// srv.executeAllOFF()
//-------------------------------------
    msg2monitor := "["
    srv.mut.RLock()
    for id, dev := range srv.device_index {
        if dev.uid != "" && dev.uid == id && dev.Name != "" {
            topic := Z2M + id	// формируем топик для подписки
            if err := srv.subscribe(topic, dev.qos); err != nil {	// Подписываемся на события.
                log.Println("ERROR", topic, err.Error())
            }else{
                log.Println("Subscribed to topic:", topic, " Qos:", dev.qos)//, " Type:",dev.Type, ":",dev.Ptrs, " Name:",dev.Name)
            }
            msg2monitor += fmt.Sprintf(`{"uid":"%s","name":"%s","qos":%d,"exec":%v},`, dev.uid, dev.Name, dev.qos, dev.executor)
        }
    }
    srv.mut.RUnlock()

    if len(msg2monitor) > 22 {
        msg2monitor = msg2monitor[:len(msg2monitor)-1]		// удалим последнюю запятую
        msg2monitor += "]"
        log.Println("4Monitor:", msg2monitor)
    }
//-------------------------------------

    srv.automatic = true		// Включить автоматику!
    go srv.checkStatus()		// Мониторинг состояния

    for{				// Ожидание событий и запуск процесса исполнения
        time.Sleep(time.Millisecond * time.Duration(10))
        if sev := <- srv.sensor_event; sev != nil && sev.uid != "" {
            srv.executeRules(sev)	// Проверить связи и правила для события и исполнить задания
        }
    } // for безусловный
} // end

//===========================================================================
