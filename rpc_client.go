package synapse

import (
	"log"
	"github.com/bitly/go-simplejson"
	"github.com/streadway/amqp"
	"time"
)
/**
绑定RPC Callback监听队列
 */
func (s *Server) rpcCallbackQueue() {
	q, err := s.mqch.QueueDeclare(
		s.SysName + "_rpc_cli_" + s.AppName + "_" + s.AppId, // name
		true, // durable
		true, // delete when usused
		true, // exclusive
		false, // no-wait
		nil, // arguments
	)
	s.failOnError(err, "Failed to declare rpcQueue")

	err = s.mqch.QueueBind(
		q.Name,
		"rpc.cli." + s.AppName + "." + s.AppId,
		s.SysName,
		false,
		nil)
	s.failOnError(err, "Failed to Bind Rpc Exchange and Queue")
}

/**
创建 Callback 队列监听
 */
func (s *Server) rpcCallbackQueueListen() {
	s.cli, err = s.mqch.Consume(
		s.SysName + "_rpc_cli_" + s.AppName + "_" + s.AppId, // queue
		s.SysName + "." + s.AppName + ".rpc.cli." + s.AppId, // consumer
		true, // auto-ack
		true, // exclusive
		false, // no-local
		false, // no-wait
		nil, // args
	)
	s.failOnError(err, "Failed to register Rpc Callback consumer")
}

/**
RPC Clenit
 */
func (s *Server) rpcClient(data map[string]interface{}, corrId string) {
	query := simplejson.New();
	query.Set("from", s.AppName + "." + s.AppId)
	query.Set("to", data["appName"].(string))
	query.Set("action", data["action"])
	query.Set("params", data["params"])
	queryJson, _ := query.MarshalJSON()
	err = s.mqch.Publish(
		s.SysName, // exchange
		"rpc.srv." + data["appName"].(string), // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			CorrelationId: corrId,
			ReplyTo:       "rpc.cli." + s.AppName + "." + s.AppId,
			Body:          []byte(queryJson),
		})
	s.failOnError(err, "Failed to publish Rpc Request")
	if s.Debug {
		log.Printf("[Synapse Debug] Publish Rpc Request: %s", queryJson)
	}
	for d := range s.cli {
		_, haveKey := s.cliResMap[d.CorrelationId]
		if haveKey {
			query, _ := simplejson.NewJson(d.Body)
			if s.Debug {
				logData, _ := query.MarshalJSON()
				log.Printf("[Synapse Debug] Receive Rpc Callback: %s", logData)
			}
			s.cliResMap[d.CorrelationId] <- query.Get("params")
			break
		}
	}
}

/**
发起 RPC请求
 */
func (s *Server) SendRpc(appName, action string, params map[string]interface{}) *simplejson.Json {
	if s.DisableRpcClient {
		log.Printf("[Synapse Error] %s: %s \n", "Rpc Request Not Send", "DisableRpcClient set true")
		ret := simplejson.New();
		ret.Set("code", 404)
		ret.Set("message", "Rpc Request Not Send: DisableRpcClient set true")
		return ret
	}
	data := map[string]interface{}{
		"appName": appName,
		"action": action,
		"params": params,
	}
	corrId := s.randomString(20)
	s.cliResMap[corrId] = make(chan *simplejson.Json)
	go s.rpcClient(data, corrId)
	select {
	case ret := <-s.cliResMap[corrId]:
		delete(s.cliResMap, corrId)
		return ret
	case <-time.After(time.Second * s.RpcTimeout):
		delete(s.cliResMap, corrId)
		log.Printf("[Synapse Error] %s: %s \n", "Rpc Request Not Success", "Request Timeout")
		ret := simplejson.New();
		ret.Set("code", 504)
		ret.Set("message", "Rpc Request Not Success: Request Timeout")
		return ret
	}

}