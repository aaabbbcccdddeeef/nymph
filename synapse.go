package synapse

import (
	"github.com/streadway/amqp"
	"time"
	"math/rand"
	"runtime"
	"reflect"
	"github.com/bitly/go-simplejson"
	"fmt"
)

type Server struct {
	Debug              bool
	DisableRpcClient   bool
	DisableEventClient bool
	AppName            string
	AppId              string
	SysName            string
	MqHost             string
	MqPort             string
	MqUser             string
	MqPass             string
	MqVHost            string
	ProcessNum         int
	EventCallback      map[string]func(*simplejson.Json, amqp.Delivery) bool
	RpcCallback map[string]func(*simplejson.Json, amqp.Delivery) map[string]interface{}
	RpcTimeout time.Duration

	conn      *amqp.Connection
	mqch      *amqp.Channel
	cli       <-chan amqp.Delivery
	cliResMap map[string]chan *simplejson.Json
}

const LogInfo = "Info"
const LogDebug = "Debug"
const LogWarn = "Warn"
const LogError = "Error"

var err error

/**
创建一个新的Synapse
 */
func New() *Server {
	return &Server{}
}

/**
启动 Synapse 组件, 开始监听RPC请求和事件
 */
func (s *Server) Serve() {
	if s.AppName == "" || s.SysName == "" {
		Log("Must Set SysName and AppName system", LogError)
		return;
	} else {
		Log(fmt.Sprintf("System Name: %s", s.SysName), LogInfo)
		Log(fmt.Sprintf("App Name: %s", s.AppName), LogInfo)
	}
	if s.AppId == "" {
		s.AppId = s.randomString(20)
	}
	Log(fmt.Sprintf("App ID: %s", s.AppId), LogInfo)
	if s.Debug {
		Log("App Run Mode: Debug", LogDebug)
	} else {
		Log("App Run Mode: Production", LogInfo)
	}
	if s.ProcessNum == 0 {
		s.ProcessNum = 80
	}
	goto START
START:
	s.createConnection()
	defer s.conn.Close()
	s.createChannel()
	time.Sleep(time.Second * 2)
	s.checkAndCreateExchange()
	if s.EventCallback != nil {
		go s.eventServer()
		for k, v := range s.EventCallback {
			Log(fmt.Sprintf("*EVT: %s -> %s", k, runtime.FuncForPC(reflect.ValueOf(v).Pointer()).Name()), LogInfo)
		}
	} else {
		Log("Event Server Disabled: EventCallback not set", LogWarn)
	}
	if s.RpcCallback != nil {
		go s.rpcServer()
		for k, v := range s.RpcCallback {
			Log(fmt.Sprintf("*RPC: %s -> %s", k, runtime.FuncForPC(reflect.ValueOf(v).Pointer()).Name()), LogInfo)
		}
	} else {
		Log("Rpc Server Disabled: RpcCallback not set", LogWarn)
	}
	if s.DisableEventClient {
		Log("Event Client Disabled: DisableEventClient set true", LogWarn)
	} else {
		Log("Event Client Ready", LogInfo)
	}
	if !s.DisableRpcClient {
		s.cliResMap = make(map[string]chan *simplejson.Json)
		s.rpcCallbackQueue()
		go s.rpcCallbackQueueListen()
		if s.RpcTimeout == 0 {
			s.RpcTimeout = 3
		}
	} else {
		Log("Rpc Client Disabled: DisableRpcClient set true", LogWarn)
	}
	var closedConnChannel = s.conn.NotifyClose(make(chan *amqp.Error))
	Log(fmt.Sprintf("Connection Error: %s , reconnect after 5 sec", <-closedConnChannel), LogError)
	time.Sleep(5 * time.Second)
	goto START
}

/**
创建到 Rabbit MQ的链接
 */
func (s *Server) createConnection() {
	s.conn, err = amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%s/%s", s.MqUser, s.MqPass, s.MqHost, s.MqPort, s.MqVHost))
	if err != nil {
		Log(fmt.Sprintf("Failed to connect to RabbitMQ: %s", err), LogError)
	}
	Log("Rabbit MQ Connection Created.", LogInfo)
}

/**
创建到 Rabbit MQ 的通道
 */
func (s *Server) createChannel() {
	s.mqch, err = s.conn.Channel()
	if err != nil {
		Log(fmt.Sprintf("Failed to open a channel: %s", err), LogError)
	}
	if s.ProcessNum > 0 {
		s.mqch.Qos(
			s.ProcessNum, // prefetch count
			0,            // prefetch size
			false,        // global
		)
	}
	Log("RabbitMQ Channel Created.", LogInfo)
	Log(fmt.Sprintf("Server MaxProcessNum: %d", s.ProcessNum), LogInfo)
}

/**
注册通讯用的 MQ Exchange
 */
func (s *Server) checkAndCreateExchange() {
	err := s.mqch.ExchangeDeclare(
		s.SysName, // name
		"topic",   // type
		true,      // durable
		true,      // auto-deleted
		false,     // internal
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		Log(fmt.Sprintf("Failed to declare Exchange: %s", err), LogError)
	}
	Log("Register Exchange Successed.", LogInfo)
}

/**
生成随机字符串
 */
func (s *Server) randomString(l int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < l; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}

/**
控制台日志
 */
func Log(desc string, level string) {
	fmt.Printf("[%s][Synapse %s] %s \n", time.Now().Format("2006-01-02 15:04:05"), level, desc)
}
