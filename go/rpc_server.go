package synapse

import (
	"github.com/streadway/amqp"
	"log"
	"github.com/bitly/go-simplejson"
	"net/http"
)

/**
绑定RPC监听队列
 */
func (s *Server) rpcQueue() {
	q, err := s.mqch.QueueDeclare(
		s.SysName + "_rpc_srv_" + s.AppName, // name
		true, // durable
		true, // delete when usused
		false, // exclusive
		false, // no-wait
		nil, // arguments
	)
	s.failOnError(err, "Failed to declare rpcQueue")

	err = s.mqch.QueueBind(
		q.Name,
		"rpc.srv." + s.AppName,
		s.SysName,
		false,
		nil)
	s.failOnError(err, "Failed to Bind Rpc Exchange and Queue")

	err = s.mqch.Qos(
		1, // prefetch count
		0, // prefetch size
		false, // global
	)
	s.failOnError(err, "Failed to set Rpc Queue QoS")
}

/**
创建结果返回json包源
 */
func (s *Server) makeRet(code int) map[string]interface{} {
	return map[string]interface{}{
		"code": code,
		"message": http.StatusText(code),
	}
}

/**
创建RPC监听
callback回调为监听到RPC请求后的处理函数
 */
func (s *Server) rpcServer() {
	s.rpcQueue()
	msgs, err := s.mqch.Consume(
		s.SysName + "_rpc_srv_" + s.AppName, // queue
		"", // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil, // args
	)
	s.failOnError(err, "Failed to register rpcServer consumer")
	log.Printf("[Synapse Info] Rpc Server Handler Listening")
	for d := range msgs {
		query, _ := simplejson.NewJson(d.Body)
		action := query.Get("action").MustString()
		params := query.Get("params").MustMap()
		if s.Debug {
			logData, _ := query.MarshalJSON()
			log.Printf("[Synapse Debug] Receive Rpc Request: %s", logData)
		}
		var resultSource map[string]interface{}
		result := s.makeRet(200)
		callback, ok := s.RpcCallbackMap[action]
		if ok {
			resultSource = callback(params, d)

		} else {
			resultSource = map[string]interface{}{"code":404, "message": "The Rpc Action Not Found"}
		}
		_, haveMessage := resultSource["message"]
		_, haveCode := resultSource["code"]
		if !haveMessage && haveCode {
			resultSource["message"] = http.StatusText(resultSource["code"].(int))
		}
		for k, v := range resultSource {
			result[k] = v
		}
		response := simplejson.New();
		response.Set("from", s.AppName + "." + s.AppId)
		response.Set("to", query.Get("from").MustString())
		response.Set("action", "reply-" + action)
		response.Set("params", result)
		resultJson, _ := response.MarshalJSON()
		err = s.mqch.Publish(
			s.SysName, // exchange
			d.ReplyTo, // routing key
			false, // mandatory
			false, // immediatec
			amqp.Publishing{
				ContentType:   "application/json",
				CorrelationId: d.CorrelationId,
				Body:          []byte(resultJson),
			})
		s.failOnError(err, "Failed to reply Rpc Request")
		if s.Debug {
			log.Printf("[Synapse Debug] Reply Rpc Request: %s", resultJson)
		}
		d.Ack(false)
	}
}