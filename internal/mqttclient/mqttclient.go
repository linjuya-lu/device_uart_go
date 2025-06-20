// internal/mqtt/client.go
package mqttclient

import (
	"fmt"
	"time"

	"encoding/json"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/google/uuid"
)

// NewClient 根据传入的 broker URL 和 clientID 创建并连接 MQTT 客户端
func NewClient(brokerURL, clientID string) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		// 可选：设置自动重连，心跳，超时等
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetKeepAlive(60 * time.Second).
		SetPingTimeout(10 * time.Second)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if ok := token.WaitTimeout(10 * time.Second); !ok {
		return nil, fmt.Errorf("MQTT 连接超时")
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("MQTT 连接失败: %w", err)
	}
	return client, nil
}

// EdgexMessage 是 EdgeX MessageBus 的通用消息格式
type EdgexMessage struct {
	ApiVersion    string      `json:"apiVersion"`
	ReceivedTopic string      `json:"receivedTopic,omitempty"`
	CorrelationID string      `json:"correlationID"`
	RequestID     string      `json:"requestID"`
	ErrorCode     int         `json:"errorCode"`
	Payload       interface{} `json:"payload,omitempty"`
	ContentType   string      `json:"contentType"`
}

// SerialPayload 是 payload 部分的结构
type SerialPayload struct {
	Port      string `json:"port"`
	Timestamp int64  `json:"timestamp"` // Unix 纳秒
	Data      string `json:"data"`      // 这里用 Base64 编码原始二进制
}

// PublishSerialFrame 组装并发布一条 EdgeX 格式的消息：
//   - topic: 要发布的 MQTT 主题
//   - port:  串口设备节点，如 "/dev/ttyUSB1"
//   - frame: 串口读到的原始 []byte 数据
func PublishSerialFrame(client mqtt.Client, topic, port string, frame []byte) error {
	// 1. 内层 payload
	payload := SerialPayload{
		Port:      port,
		Timestamp: time.Now().UnixNano(),
		Data:      string(frame),
	}

	// 2. 外层通用消息
	msg := EdgexMessage{
		ApiVersion:    "v3",
		ReceivedTopic: "", // 如果发布时想填入实际接收主题可在调用处赋值
		CorrelationID: uuid.NewString(),
		RequestID:     uuid.NewString(),
		ErrorCode:     0,
		Payload:       payload,
		ContentType:   "application/json",
	}

	// 3. 序列化 JSON
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// 4. 发布并等待完成
	tok := client.Publish(topic, 0, false, body)
	tok.Wait()
	return tok.Error()
}
