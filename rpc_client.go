package synapse

import (
	"github.com/bitly/go-simplejson"
	"github.com/streadway/amqp"
	"time"
	"fmt"
	"encoding/json"
)

/**
绑定RPC Callback监听队列
 */
func (s *Server) rpcCallbackQueue() {
	q, err := s.mqch.QueueDeclare(
		fmt.Sprintf("%s_%s_client_%s", s.SysName, s.AppName, s.AppId),
		true,  // durable
		true,  // delete when usused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		Log(fmt.Sprintf("Failed to declare rpcQueue: %s", err), LogError)
	}
	err = s.mqch.QueueBind(
		q.Name,
		fmt.Sprintf("client.%s.%s", s.AppName, s.AppId),
		s.SysName,
		false,
		nil)
	if err != nil {
		Log(fmt.Sprintf("Failed to Bind Rpc Exchange and Queue: %s", err), LogError)
	}
}

/**
创建 Callback 队列监听
 */
func (s *Server) rpcCallbackQueueListen() {
	s.cli, err = s.mqch.Consume(
		fmt.Sprintf("%s_%s_client_%s", s.SysName, s.AppName, s.AppId), // queue
		fmt.Sprintf("client.%s.%s", s.AppName, s.AppId),               // consumer
		true,                                                          // auto-ack
		true,                                                          // exclusive
		false,                                                         // no-local
		false,                                                         // no-wait
		nil,                                                           // args
	)
	if err != nil {
		Log(fmt.Sprintf("Failed to register Rpc Callback consumer: %s", err), LogError)
	}
	Log(fmt.Sprintf("Rpc Client Timeout: %d", s.RpcTimeout), LogInfo)
	Log("Rpc Client Ready", LogInfo)
}

/**
RPC Clenit
 */
func (s *Server) rpcClient(appName, action string, params map[string]interface{}, msgId string) {
	query, _ := json.Marshal(params)
	err = s.mqch.Publish(
		s.SysName,         // exchange
		"server."+appName, // routing key
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			AppId:     s.AppId,
			MessageId: msgId,
			ReplyTo:   s.AppName,
			Type:      action,
			Body:      query,
		})
	if err != nil {
		Log(fmt.Sprintf("Failed to publish Rpc Request: %s", err), LogError)
	}
	if s.Debug {
		Log(fmt.Sprintf("RPC Request: (%s)%s->%s@%s %s", msgId, s.AppName, action, appName, query), LogDebug)
	}
	for d := range s.cli {
		_, haveKey := s.cliResMap[d.CorrelationId]
		if haveKey {
			query, _ := simplejson.NewJson(d.Body)
			if s.Debug {
				logData, _ := query.MarshalJSON()
				Log(fmt.Sprintf("RPC Response: (%s)%s@%s->%s %s", d.CorrelationId, d.Type, d.ReplyTo, s.AppName, logData), LogDebug)
			}
			s.cliResMap[d.CorrelationId] <- query
			break
		}
	}
}

/**
发起 RPC请求
 */
func (s *Server) SendRpc(appName, action string, params map[string]interface{}) *simplejson.Json {
	if s.DisableRpcClient {
		Log("Rpc Client Disabled: DisableRpcClient set true", LogError)
		ret := simplejson.New()
		ret.Set("rpc_error", "rpc client disabled")
		return ret
	}
	msgId := s.randomString(20)
	s.cliResMap[msgId] = make(chan *simplejson.Json)
	go s.rpcClient(appName, action, params, msgId)
	select {
	case ret := <-s.cliResMap[msgId]:
		delete(s.cliResMap, msgId)
		return ret
	case <-time.After(time.Second * s.RpcTimeout):
		delete(s.cliResMap, msgId)
		ret := simplejson.New()
		ret.Set("rpc_error", "timeout")
		return ret
	}

}
