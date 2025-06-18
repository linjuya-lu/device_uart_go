// internal/driver/init.go
package driver

import (
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/linjuya-lu/device_uart_go/internal/config"
	"github.com/linjuya-lu/device_uart_go/internal/serial"
)

// InitializeSerialProxy 负责：
//  1. 加载配置
//  2. 打开所有串口
//  3. 为每个串口启动带帧解析的读循环
//  4. 订阅所有协议的命令主题，把收到的命令写到对应串口
func InitializeSerialProxy(configPath string, mqttClient mqtt.Client) error {
	// 1. 载入 YAML
	if err := config.LoadConfig(configPath); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. 打开所有串口并记入 portMap
	portMap := make(map[string]serial.Port, len(config.SerialCfg.Ports))
	for _, pc := range config.SerialCfg.Ports {
		p, err := serial.NewPort(pc)
		if err != nil {
			return fmt.Errorf("unsupported port %s: %w", pc.Name, err)
		}
		if err := p.Open(); err != nil {
			return fmt.Errorf("open port %s: %w", pc.Name, err)
		}
		portMap[pc.Name] = p
	}

	// 3. 根据 Bindings 构建 port→protocol 映射
	portProto := make(map[string]string, len(config.SerialCfg.Bindings))
	for _, b := range config.SerialCfg.Bindings {
		portProto[b.PortName] = b.ProtocolID
	}

	// 4. 为每个端口启动带解析器的读循环
	for portName, p := range portMap {
		// 选协议 ID，找不到则用默认
		protoID := portProto[portName]
		if protoID == "" {
			protoID = config.SerialCfg.DefaultProtocol
		}
		// 拿解析器
		fp, ok := serial.Parsers[protoID]
		if !ok {
			return fmt.Errorf("no parser for protocol %s", protoID)
		}
		// 拿 responseTopic
		pr := findProtocolByID(protoID)
		var respTopic string
		if pr != nil {
			respTopic = pr.ResponseTopic
		}

		// 启动解析发布 loop
		go func(port serial.Port, parse serial.FrameParser, topic string) {
			var buf []byte
			tmp := make([]byte, 256)
			for {
				n, err := port.Read(tmp)
				if err != nil {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				buf = append(buf, tmp[:n]...)
				for {
					frame, rest, err := parse(buf)
					if err != nil {
						// 出错直接丢弃整个缓存，重开
						buf = nil
						break
					}
					if frame == nil {
						// 未组成完整帧，留着下次继续
						break
					}
					// 发布到 MQTT
					if topic != "" {
						mqttClient.Publish(topic, 0, false, frame)
					}
					buf = rest
				}
			}
		}(p, fp, respTopic)
	}

	// 5. 订阅所有协议的 requestTopic，把收到的 payload 写到对应串口
	for _, pr := range config.SerialCfg.Protocols {
		topic := pr.RequestTopic
		id := pr.ID
		mqttClient.Subscribe(topic, 0, func(_ mqtt.Client, msg mqtt.Message) {
			for portName, boundID := range portProto {
				if boundID != id {
					continue
				}
				if p, ok := portMap[portName]; ok {
					p.Write(msg.Payload())
				}
			}
		})
	}

	return nil
}

// findProtocolByID 根据协议 ID 查回配置项
func findProtocolByID(id string) *config.Protocol {
	for i := range config.SerialCfg.Protocols {
		if config.SerialCfg.Protocols[i].ID == id {
			return &config.SerialCfg.Protocols[i]
		}
	}
	return nil
}
