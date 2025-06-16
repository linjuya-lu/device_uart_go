package mqtt

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// ClientOptions 配置 MQTT 客户端行为
// Broker: tcp://host:port
// ClientID: 客户端标识
// Username/Password: 可选认证
// KeepAlive: 心跳间隔
// ConnectTimeout: 连接超时
// DefaultQos/DefaultRetain: 默认发布参数
type ClientOptions struct {
	Broker         string
	ClientID       string
	Username       string
	Password       string
	KeepAlive      time.Duration
	ConnectTimeout time.Duration
	DefaultQos     byte
	DefaultRetain  bool
}

// Client 封装 Paho MQTT 客户端，支持三类消息流：
// 业务数据（纯二进制）、控制命令（JSON）、命令反馈（JSON）
type Client struct {
	inner paho.Client
	opts  ClientOptions
	mu    sync.Mutex
}

// NewClient 创建一个新的 MQTT 客户端并连接到 Broker
func NewClient(opts ClientOptions) (*Client, error) {
	p := paho.NewClientOptions().
		AddBroker(opts.Broker).
		SetClientID(opts.ClientID).
		SetKeepAlive(opts.KeepAlive).
		SetCleanSession(true)
	if opts.Username != "" {
		p.SetUsername(opts.Username)
	}
	if opts.Password != "" {
		p.SetPassword(opts.Password)
	}
	c := &Client{opts: opts}
	c.inner = paho.NewClient(p)
	tok := c.inner.Connect()
	if !tok.WaitTimeout(opts.ConnectTimeout) {
		return nil, fmt.Errorf("mqtt connect timeout after %s", opts.ConnectTimeout)
	}
	if err := tok.Error(); err != nil {
		return nil, err
	}
	return c, nil
}

// PublishData 发布业务数据（纯二进制）到指定主题
func (c *Client) PublishData(topic string, data []byte) error {
	tok := c.inner.Publish(topic, c.opts.DefaultQos, c.opts.DefaultRetain, data)
	tok.Wait()
	return tok.Error()
}

// SubscribeData 订阅业务数据主题，handler 接收原始二进制数据
func (c *Client) SubscribeData(topic string, handler func([]byte)) error {
	tok := c.inner.Subscribe(topic, c.opts.DefaultQos, func(_ paho.Client, m paho.Message) {
		handler(m.Payload())
	})
	tok.Wait()
	return tok.Error()
}

// PublishCommand 发布控制命令（JSON）到指定主题
func (c *Client) PublishCommand(topic string, cmd interface{}) error {
	b, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("marshal command: %w", err)
	}
	tok := c.inner.Publish(topic, c.opts.DefaultQos, c.opts.DefaultRetain, b)
	tok.Wait()
	return tok.Error()
}

// SubscribeCommand 订阅控制命令主题，handler 接收原始 JSON 数据
func (c *Client) SubscribeCommand(topic string, handler func([]byte)) error {
	tok := c.inner.Subscribe(topic, c.opts.DefaultQos, func(_ paho.Client, m paho.Message) {
		handler(m.Payload())
	})
	tok.Wait()
	return tok.Error()
}

// PublishResponse 发布命令反馈（JSON）到指定主题
func (c *Client) PublishResponse(topic string, resp interface{}) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	tok := c.inner.Publish(topic, c.opts.DefaultQos, c.opts.DefaultRetain, b)
	tok.Wait()
	return tok.Error()
}

// SubscribeResponse 订阅命令反馈主题，handler 接收原始 JSON 数据
func (c *Client) SubscribeResponse(topic string, handler func([]byte)) error {
	tok := c.inner.Subscribe(topic, c.opts.DefaultQos, func(_ paho.Client, m paho.Message) {
		handler(m.Payload())
	})
	tok.Wait()
	return tok.Error()
}

// Disconnect 断开与 Broker 的连接
func (c *Client) Disconnect(quiesce uint) {
	c.inner.Disconnect(quiesce)
}

// func main() {
//     // 1) 创建并连接
//     opts := mqtt.ClientOptions{
//         Broker:         "tcp://127.0.0.1:1883",
//         ClientID:       "serial-proxy-1",
//         KeepAlive:      30 * time.Second,
//         ConnectTimeout: 5 * time.Second,
//         DefaultQos:     1,
//         DefaultRetain:  false,
//     }
//     cli, err := mqtt.NewClient(opts)
//     if err != nil {
//         panic(err)
//     }
//     defer cli.Disconnect(250)

//     // 假设 deviceName, protocol 由配置得到
//     deviceName := "Meter001"
//     protocol   := "IEC101"

//     // 2) 发布业务数据（二进制）
//     dataTopic := fmt.Sprintf("edgex/serialproxy/%s/%s/data", deviceName, protocol)
//     rawFrame  := []byte{0x68, 0x03, 0x03, 0x68, 0x00, 0x16} // 示例帧
//     if err := cli.PublishData(dataTopic, rawFrame); err != nil {
//         fmt.Println("PublishData failed:", err)
//     }

//     // 3) 订阅业务数据
//     cli.SubscribeData(dataTopic, func(payload []byte) {
//         fmt.Printf("收到业务数据：% X\n", payload)
//     })

//     // 4) 发布控制命令（JSON）
//     cmdTopic := fmt.Sprintf("edgex/serialproxy/%s/%s/control", deviceName, protocol)
//     cmd := map[string]interface{}{
//         "correlationID": "req-123",
//         "command":       "setTime",
//         "params": map[string]int{
//             "year":  2025,
//             "month": 6,
//             "day":   13,
//         },
//     }
//     if err := cli.PublishCommand(cmdTopic, cmd); err != nil {
//         fmt.Println("PublishCommand failed:", err)
//     }

//     // 5) 订阅控制命令
//     cli.SubscribeCommand(cmdTopic, func(raw []byte) {
//         fmt.Println("收到控制命令 JSON:", string(raw))
//     })

//     // 6) 发布命令反馈（JSON）
//     respTopic := fmt.Sprintf("edgex/serialproxy/%s/%s/control/response", deviceName, protocol)
//     resp := map[string]interface{}{
//         "correlationID": "req-123",
//         "status":        "OK",
//         "message":       "Time set successfully",
//     }
//     if err := cli.PublishResponse(respTopic, resp); err != nil {
//         fmt.Println("PublishResponse failed:", err)
//     }

//     // 7) 订阅命令反馈
//     cli.SubscribeResponse(respTopic, func(raw []byte) {
//         fmt.Println("收到命令反馈 JSON:", string(raw))
//     })

//     // 保持运行，看回调输出
//     select {}
// }
