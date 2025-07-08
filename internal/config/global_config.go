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
	SerialCfg   *SerialProxyConfig
	once        sync.Once
	portMap     map[string]Port
	ProtocolMap map[string]Protocol   // 协议 ID → Protocol
	bindingMap  map[string][]Protocol // 端口名 → 多个 Protocol
)

// 指定 YAML 文件加载配置
func LoadConfig(path string) error {
	var err error
	once.Do(func() {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			err = readErr
			return
		}
		cfg := struct {
			SerialProxy SerialProxyConfig `yaml:"SerialProxy"`
		}{}
		if unErr := yaml.Unmarshal(data, &cfg); unErr != nil {
			err = unErr
			return
		}
		SerialCfg = &cfg.SerialProxy

		// 构建 portMap
		portMap = make(map[string]Port, len(SerialCfg.Ports))
		for _, p := range SerialCfg.Ports {
			portMap[p.Name] = p
		}

		// 构建 protocolMap
		ProtocolMap = make(map[string]Protocol, len(SerialCfg.Bindings))
		for _, b := range SerialCfg.Bindings {
			id := b.ProtocolID
			if _, exists := ProtocolMap[id]; !exists {
				req := fmt.Sprintf(
					"edgex/service/command/request/device_uart/%s", id)
				rsp := fmt.Sprintf(
					"edgex/service/data/device_uart/%s", id)
				ProtocolMap[id] = Protocol{ID: id, RequestTopic: req, ResponseTopic: rsp}
			}
		}

		// 构建 bindingMap（端口 → 多个协议）
		bindingMap = make(map[string][]Protocol, len(SerialCfg.Bindings))
		for _, b := range SerialCfg.Bindings {
			if pr, ok := ProtocolMap[b.ProtocolID]; ok {
				bindingMap[b.PortName] = append(bindingMap[b.PortName], pr)
			}
		}
		// **新增：** 同步把 protocolMap 转成 SerialCfg.Protocols
		SerialCfg.Protocols = make([]Protocol, 0, len(ProtocolMap))
		for _, pr := range ProtocolMap {
			SerialCfg.Protocols = append(SerialCfg.Protocols, pr)
		}
	})
	return err
}

// GetProtocolsForPort 返回指定端口对应的所有协议，未绑定则返回默认协议
func GetProtocolsForPort(name string) []Protocol {
	if ps, ok := bindingMap[name]; ok && len(ps) > 0 {
		return ps
	}
	// 如果没有绑定，返回默认协议
	if def, ok := ProtocolMap[SerialCfg.DefaultProtocol]; ok {
		return []Protocol{def}
	}
	return nil
}

// GetPort 根据端口名称返回 Port 配置
func GetPort(name string) (Port, bool) {
	p, ok := portMap[name]
	return p, ok
}
