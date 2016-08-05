package synapse

import (
	"github.com/streadway/amqp"
	"log"
	"time"
	"math/rand"
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
	ProcessNum         int
	EventCallback      map[string]func(map[string]interface{}, amqp.Delivery) bool
	RpcCallback        map[string]func(map[string]interface{}, amqp.Delivery) map[string]interface{}
	RpcTimeout         time.Duration

	conn               *amqp.Connection
	mqch               *amqp.Channel
	cli                <-chan amqp.Delivery
	cliResMap          map[string]chan map[string]interface{}
}

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
		log.Fatalf("[Synapse Error] Must Set SysName and AppName system exit .")
	} else {
		log.Print("[Synapse Info] System Name: ", s.SysName)
		log.Print("[Synapse Info] App Name: ", s.AppName)
	}
	if s.ProcessNum == 0 {
		s.ProcessNum = 100
	}
	log.Print("[Synapse Info] App MaxProcessNum: ", s.ProcessNum)
	if s.AppId == "" {
		s.AppId = s.randomString(20)
	}
	log.Print("[Synapse Info] App ID: ", s.AppId)
	if s.Debug {
		log.Print("[Synapse Warn] App Run Mode: Debug")
	} else {
		log.Print("[Synapse Info] App Run Mode: Production")
	}
	goto START
	START:
	s.createConnection()
	defer s.conn.Close()
	s.createChannel()
	defer s.mqch.Close()
	time.Sleep(time.Second * 2)
	s.checkAndCreateExchange()
	if s.EventCallback != nil {
		go s.eventServer()
	} else {
		log.Printf("[Synapse Warn] Event Handler Disabled: EventCallbackMap not set")
	}
	if s.RpcCallback != nil {
		go s.rpcServer()
	} else {
		log.Printf("[Synapse Warn] Rpc Handler Disabled: RpcCallbackMap not set")
	}
	if s.DisableEventClient {
		log.Printf("[Synapse Warn] Event Sender Disabled: DisableEventClient set true")
	} else {
		log.Printf("[Synapse Info] Event Sender Ready")
	}
	if !s.DisableRpcClient {
		s.cliResMap = make(map[string]chan map[string]interface{})
		s.rpcCallbackQueue()
		go s.rpcCallbackQueueListen()
		if s.RpcTimeout == 0 {
			s.RpcTimeout = 3
		}
	} else {
		log.Printf("[Synapse Warn] Rpc Sender Disabled: DisableRpcClient set true")
	}
	var closedConnChannel = s.conn.NotifyClose(make(chan *amqp.Error))
	log.Printf("[Synapse Error] Connection Error: %s , reconnect after 5 sec", <-closedConnChannel)
	time.Sleep(5 * time.Second)
	goto START
}

/**
创建到 Rabbit MQ的链接
 */
func (s *Server) createConnection() {
	s.conn, err = amqp.Dial("amqp://" + s.MqUser + ":" + s.MqPass + "@" + s.MqHost + ":" + s.MqPort)
	s.failOnError(err, "Failed to connect to RabbitMQ")
	log.Print("[Synapse Info] Rabbit MQ Connection Created.")
}

/**
创建到 Rabbit MQ 的通道
 */
func (s *Server) createChannel() {
	s.mqch, err = s.conn.Channel()
	s.failOnError(err, "Failed to open a channel")
	err = s.mqch.Qos(
		s.ProcessNum, // prefetch count
		0, // prefetch size
		false, // global
	)
	s.failOnError(err, "Failed to set Rpc Queue QoS")
	log.Print("[Synapse Info] Rabbit MQ Channel Created.")
}

/**
注册通讯用的 MQ Exchange
 */
func (s *Server) checkAndCreateExchange() {
	err := s.mqch.ExchangeDeclare(
		s.SysName, // name
		"topic", // type
		true, // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil, // arguments
	)
	s.failOnError(err, "Failed to declare Event Exchange")
	return
}

/**
便捷报错方法
 */
func (s *Server) failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("[Synapse Error] %s: %s \n", msg, err)
	}
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
