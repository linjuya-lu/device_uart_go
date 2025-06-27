package config

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v2"
)

type SerialConfig struct {
	SerialID  string `yaml:"SerialID"`
	Baudrate  int    `yaml:"Baudrate"`
	DEPin     int    `yaml:"DEPin"`       // RS-485 DE/RE 控制 GPIO 编号
	TimeoutMs int    `yaml:"ReadTimeout"` // 毫秒
}

var (
	// SerialCfg 全局持有反序列化后的配置
	SerialCfg *SerialProxyConfig
	once      sync.Once

	// 内存“表”
	portMap     map[string]Port
	protocolMap map[string]Protocol // 协议 ID → Protocol
	bindingMap  map[string]Protocol // 端口名 → Protocol
)

// LoadConfig 从指定 YAML 文件加载配置，只初始化一次
func LoadConfig(path string) error {
	var err error
	once.Do(func() {
		// 1. 读取文件
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			err = readErr
			return
		}

		// 2. 反序列化到 SerialProxyConfig
		cfg := struct {
			SerialProxy SerialProxyConfig `yaml:"SerialProxy"`
		}{}
		if unErr := yaml.Unmarshal(data, &cfg); unErr != nil {
			err = unErr
			return
		}
		SerialCfg = &cfg.SerialProxy

		// 3. 构建 portMap
		portMap = make(map[string]Port, len(SerialCfg.Ports))
		for _, p := range SerialCfg.Ports {
			portMap[p.Name] = p
		}

		// 4. 动态构建 protocolMap —— 只扫 Bindings，不再用 SerialCfg.Protocols
		protocolMap = make(map[string]Protocol, len(SerialCfg.Bindings))
		for _, b := range SerialCfg.Bindings {
			id := b.ProtocolID
			if _, exists := protocolMap[id]; exists {
				continue
			}
			// 用固定前缀 + protocolId 拼请求/响应主题
			req := fmt.Sprintf(
				"edgex/service/command/request/device_uart/%s", id)
			rsp := fmt.Sprintf(
				"edgex/service/data/device_uart/%s", id)
			protocolMap[id] = Protocol{
				ID:            id,
				RequestTopic:  req,
				ResponseTopic: rsp,
			}
		}

		// 5. 构建 bindingMap（端口 → 协议）
		bindingMap = make(map[string]Protocol, len(SerialCfg.Bindings))
		for _, b := range SerialCfg.Bindings {
			if pr, ok := protocolMap[b.ProtocolID]; ok {
				bindingMap[b.PortName] = pr
			}
		}
	})
	return err
}

// GetPort 根据端口名称返回 Port 配置
func GetPort(name string) (Port, bool) {
	p, ok := portMap[name]
	return p, ok
}

// GetProtocolForPort 返回指定端口对应的 Protocol，
// 如果未绑定则返回 DefaultProtocol 对应的 Protocol
func GetProtocolForPort(name string) Protocol {
	if pr, ok := bindingMap[name]; ok {
		return pr
	}
	return protocolMap[SerialCfg.DefaultProtocol]
}
