package synapse

import (
	"github.com/bitly/go-simplejson"
	"strings"
	"github.com/streadway/amqp"
	"fmt"
)

/**
绑定事件监听队列
 */
func (s *Server) eventQueue() {
	q, err := s.mqch.QueueDeclare(
		fmt.Sprintf("%s_%s_event", s.SysName, s.AppName), // name
		true,                                             // durable
		true,                                             // delete when usused
		false,                                            // exclusive
		false,                                            // no-wait
		nil,                                              // arguments
	)
	if err != nil {
		Log(fmt.Sprintf("Failed to declare event queue: %s", err), LogError)
	}

	for k, _ := range s.EventCallback {
		err = s.mqch.QueueBind(
			q.Name,     // queue name
			"event."+k, // routing key
			s.SysName,  // exchange
			false,
			nil)
		if err != nil {
			Log(fmt.Sprintf("Failed to bind event queue: %s", k), LogError)
		}
	}
}

/**
创建事件监听
callback回调为监听到事件后的处理函数
 */
func (s *Server) eventServer() {
	s.eventQueue()
	msgs, err := s.mqch.Consume(
		fmt.Sprintf("%s_%s_event", s.SysName, s.AppName),             // queue
		fmt.Sprintf("%s.%s.event.%s", s.SysName, s.AppName, s.AppId), // consumer
		false,                                                        // auto-ack
		false,                                                        // exclusive
		false,                                                        // no-local
		false,                                                        // no-wait
		nil,                                                          // args
	)
	if err != nil {
		Log(fmt.Sprintf("Failed to register event consumer: %s", err), LogError)
	}
	for d := range msgs {
		go s.eventHandler(d)
	}
}

/**
事件处理器
 */
func (s *Server) eventHandler(d amqp.Delivery) {
	query, _ := simplejson.NewJson(d.Body)
	if s.Debug {
		logData, _ := query.MarshalJSON()
		Log(fmt.Sprintf("Event Receive: %s@%s %s", d.Type, d.ReplyTo, logData), LogDebug)
	}
	callback, ok := s.EventCallback[strings.Replace(d.RoutingKey, "event.", "", 1)]
	if ok && callback(query, d) {
		d.Ack(false)
	} else {
		d.Reject(true)
	}
}
