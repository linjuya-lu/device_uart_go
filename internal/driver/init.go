package driver

import (
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/linjuya-lu/device_uart_go/internal/config"
	"github.com/linjuya-lu/device_uart_go/internal/mqttclient"
	"github.com/linjuya-lu/device_uart_go/internal/serial"
)

// InitializeSerialProxy ：
//  1. 加载配置
//  2. 打开所有串口
//  3. 为每个串口启动单协程读循环，支持多协议解析
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
	// 3. 构建 port -> 协议ID 列表映射
	portProtoss := make(map[string][]string, len(config.SerialCfg.Bindings))
	for _, b := range config.SerialCfg.Bindings {
		portProtoss[b.PortName] = append(portProtoss[b.PortName], b.ProtocolID)
	}
	// 4. 单协程读循环：每个端口只起一个 goroutine，但支持多协议解析
	for portName, port := range portMap {
		protoIDs := portProtoss[portName]
		if len(protoIDs) == 0 {
			protoIDs = []string{config.SerialCfg.DefaultProtocol}
		}
		// 准备对应的解析器和响应主题
		parsers := make([]serial.FrameParser, 0, len(protoIDs))
		topics := make([]string, 0, len(protoIDs))
		for _, pid := range protoIDs {
			fp, ok := serial.Parsers[pid]
			if !ok {
				return fmt.Errorf("no parser for protocol %s on port %s", pid, portName)
			}
			parsers = append(parsers, fp)
			//  查找 responseTopic
			if pr, ok := config.ProtocolMap[pid]; ok {
				fmt.Printf("🔗 port=%s bind protocol=%s → responseTopic=%s\n", portName, pid, pr.ResponseTopic)
				topics = append(topics, pr.ResponseTopic)
			} else {
				fmt.Printf("⚠️ port=%s bind protocol=%s → NO Protocol found, appending empty topic\n", portName, pid)
				topics = append(topics, "")
			}
		}
		// 启动单一解析循环
		go func(p serial.Port, parsers []serial.FrameParser, topics []string, portName string) {
			var buf []byte
			tmp := make([]byte, 256)
			for {
				// 读串口数据
				n, err := p.Read(tmp)
				if err != nil {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				s := string(tmp[:n])
				fmt.Printf("⮈ [%s] Read %d bytes as string: %q\n", portName, n, s)
				buf = append(buf, tmp[:n]...)

				// 多协议匹配解析
				for {
					matched := false
					for i, parse := range parsers {
						frame, rest, err := parse(buf)
						if err != nil {
							buf = nil
							matched = false
							break
						}
						if frame != nil {
							topic := topics[i]
							fmt.Printf("→ PublishSerialFrame params: topic=%s, port=%s, frame(%d)=% X\n",
								topic, portName, len(frame), frame)
							if topic != "" {
								if err := mqttclient.PublishSerialFrame(mqttClient, topic, portName, frame); err != nil {
									fmt.Printf("❌ publish failed: %v\n", err)
								}
							}
							buf = rest
							matched = true
							break
						}
					}
					if !matched {
						break
					}
				}
			}
		}(port, parsers, topics, portName)
	}
	// 5. 订阅所有协议的 requestTopic，把收到的 JSON 解包后写到对应串口
	for _, pr := range config.SerialCfg.Protocols {

		topic := pr.RequestTopic
		fmt.Printf("🔔 Subscribing to requestTopic: %s\n", topic)
		token := mqttClient.Subscribe(topic, 0, func(_ mqtt.Client, msg mqtt.Message) {
			// 反序列化外层
			var raw map[string]interface{}
			if err := json.Unmarshal(msg.Payload(), &raw); err != nil {
				fmt.Printf("解析外层失败: %v\n", err)
				return
			}
			// 反序列化 payload
			payloadBytes, err := json.Marshal(raw["payload"])
			if err != nil {
				fmt.Printf("重编码 payload 失败: %v\n", err)
				return
			}
			var sp mqttclient.SerialPayload
			if err := json.Unmarshal(payloadBytes, &sp); err != nil {
				fmt.Printf("解析 SerialPayload 失败: %v\n", err)
				return
			}
			fmt.Printf("▶ Got request: topic=%s, raw payload=%s\n", msg.Topic(), string(msg.Payload()))
			// 写串口
			portName := sp.Port
			dataBytes := []byte(sp.Data)
			if p, ok := portMap[portName]; ok {
				fmt.Printf("⇦ 写入串口: %s, 数据=% X\n", portName, dataBytes)
				if _, err := p.Write(dataBytes); err != nil {
					fmt.Printf("写入串口 %s 失败: %v\n", portName, err)
				}
			} else {
				fmt.Printf("未找到串口 %s\n", portName)
			}
		})
		token.Wait()
		if token.Error() != nil {
			fmt.Printf("❌ 订阅 topic=%s 失败: %v\n", topic, token.Error())
		} else {
			fmt.Printf("✅ Successfully subscribed to topic=%s\n", topic)
		}
	}

	return nil
}
