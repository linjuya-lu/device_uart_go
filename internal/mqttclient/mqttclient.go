package mqttclient

import (
	"fmt"
	"strings"
	"time"

	"encoding/hex"
	"encoding/json"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/google/uuid"
)

// 根据broker URL 和 clientID 创建并连接 MQTT 客户端
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

// MessageBus消息格式
type EdgexMessage struct {
	ApiVersion    string      `json:"apiVersion"`
	ReceivedTopic string      `json:"receivedTopic,omitempty"`
	CorrelationID string      `json:"correlationID"`
	RequestID     string      `json:"requestID"`
	ErrorCode     int         `json:"errorCode"`
	Payload       interface{} `json:"payload,omitempty"`
	ContentType   string      `json:"contentType"`
}

// payload部分结构
type SerialPayload struct {
	Port      string `json:"port"`
	Timestamp int64  `json:"timestamp"` // Unix 纳秒
	Data      string `json:"data"`      // 这里用 Base64 编码原始二进制
}

// PublishSerialFrame 组装并发布一条 EdgeX 格式的消息：
func PublishSerialFrame(client mqtt.Client, topic, port string, frame []byte) error {
	// 调用入口打印原始二进制
	fmt.Printf(" PublishSerialFrame called: topic=%s, port=%s, raw frame=% X\n", topic, port, frame)
	// 十六进制字符串
	hexData := strings.ToUpper(hex.EncodeToString(frame))
	fmt.Printf(" Converted frame to hex string: %s\n", hexData)
	// 1. 内层 payload: Data 字段存放十六进制字符串
	payload := SerialPayload{
		Port:      port,
		Timestamp: time.Now().UnixNano(),
		Data:      hexData,
	}
	// 2. 外层通用消息
	msg := EdgexMessage{
		ApiVersion:    "v3",
		ReceivedTopic: "",
		CorrelationID: uuid.NewString(),
		RequestID:     uuid.NewString(),
		ErrorCode:     0,
		Payload:       payload,
		ContentType:   "application/json",
	}
	// 3. 序列化 JSON
	body, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("❌ JSON Marshal error: %v\n", err)
		return err
	}
	// 主题和消息体
	fmt.Printf("⮉ Publishing MQTT topic=%s, message=%s\n", topic, string(body))
	// 4. 发布并等待完成
	tok := client.Publish(topic, 0, false, body)
	tok.Wait()
	if err := tok.Error(); err != nil {
		fmt.Printf("❌ Publish error: %v\n", err)
	} else {
		fmt.Printf("✅ Publish succeeded: topic=%s\n", topic)
	}
	return tok.Error()
}
