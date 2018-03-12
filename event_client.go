package synapse

import (
	"github.com/streadway/amqp"
	"fmt"
	"encoding/json"
)

/**
发送一个事件
 */
func (s *Server) eventClient() {
	s.eventClientChannel = s.CreateChannel(0, "EventClient")
}
func (s *Server) SendEvent(eventName string, params map[string]interface{}) {
	if s.DisableEventClient {
		Log("Event Client Disabled: DisableEventClient set true", LogError)
	} else {
		query, _ := json.Marshal(params);
		s.eventClientChannel.Publish(
			s.SysName,                                        // exchange
			fmt.Sprintf("event.%s.%s", s.AppName, eventName), // routing key
			false,                                            // mandatory
			false,                                            // immediate
			amqp.Publishing{
				MessageId: s.randomString(20),
				AppId:     s.AppId,
				ReplyTo:   s.AppName,
				Type:      eventName,
				Body:      query,
			})
		if s.Debug {
			Log(fmt.Sprintf("Event Publish: %s@%s %s", eventName, s.AppName, query), LogDebug)
		}

	}
}
