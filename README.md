## 西纳普斯 - synapse (Golang Version)

### 此为系统核心交互组件,包含了事件和RPC系统

安装方式:

> go get code.simcu.com/jumpserver/synapse

> go get github.com/bitly/go-simplejson

> go get github.com/streadway/amqp

初始化方法:
```golang
    //创建一个服务实例
    server := synapse.New()
	//设置rpc调用方法(不设置系统将不会启动RPC服务器)
	server.RpcCallback = map[string]func(*simplejson.Json, amqp.Delivery) map[string]interface {}{
		"echo.get": echoHello,
		"echo.post": echoHello,
		"echo.put": echoHello,
		"echo.patch": echoHello,
		"echo.delete": echoHello,
	}

	//设置事件回调方法(不设置系统将不会启动事件监听器)
	server.EventCallback = map[string]func(*simplejson.Json, amqp.Delivery) bool{
		"icarus.test": test,
		"pytest.test": test,
	}
	
	//设置系统名称(相同的系统中的APP才能相互调用)
	server.SysName = "jumpserver"
	//设置应用名称(RPC调用和事件的标识)
	server.AppName = common.Config["app_name"]
	//设置RPC请求超时时间 (默认为3秒)
	server.RpcTimeout = 5
	// RabbitMQ 服务器地址
	server.MqHost = common.Config["mq_host"]
	// RabbitMQ 服务器端口
	server.MqPort = common.Config["mq_port"]
	// RabbitMQ 服务器用户
	server.MqUser = common.Config["mq_user"]
	// RabbitMQ 服务器密码
	server.MqPass = common.Config["mq_pass"]
	//组件事件和RPC最大并发请求数 (不设置默认为 100)
	server.ProcessNum = 100
	//是否禁用发送事件的机能 (默认允许发送事件)
	server.DisableEventClient = true
	//是否禁用RPC客户端功能 (默认可以进行RPC请求)
	server.DisableRpcClient = true
	//调试模式开关 (打开后可以看到很多LOG)
	server.Debug = true
	//开始服务
	server.Serve()
```

事件处理方法类型:
```golang
//第一个参数:客户端请求数据
//第二个参数:RPC传输的数据包,一般情况不使用
//需要返回 true表示处理完成,返回false表示处理失败
func(*simplejson.Json, amqp.Delivery) bool
```

RPC服务方法类型:
```golang
//第一个参数:客户端请求数据
//第二个参数:RPC传输的数据包,一般情况不使用
//需要返回: map[string]interface{}
//返回值中,包含两个可选参数 code 和 message 如果不填写,系统将会默认使用 code:200 和 message: OK, 可以手动指定
func(*simplejson.Json, amqp.Delivery) map[string]interface{}
```

发送RPC请求:
```golang
//第一个参数为要调用组件的名称
//第二个参数为要调用组件的方法
//第三个参数为map[string]interface{} 要发送的数据
//返回 *simplejson.Json

synapse.SendRpc("pytest","pyt", map[string]interface{}{"pass":1213123,"time":time.Now().String()})
```

发送一个事件:
```golang
//第一个参数为要触发的事件名称 
//第二个参数为 事件的相关数据
synapse.SendEvent("test", map[string]interface{}{"lalal":"触发了", "hahah":"is this problem"})
```
上面发送了一个名为 AppName.test的事件, 只需要在监听器中注册 AppName.test 即可在产生事件时被通知