// 文件：rs485/config.go
package rs485

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

// Config 对应原来 Emd.Agent.IEC101.E0.conf 中的 JSON 结构
type Config struct {
	SerialID string `json:"SerialID"` // 串口设备名称，例如 "/dev/ttyPS1"
	Baudrate int    `json:"Baudrate"` // 波特率，例如 115200

	// 以下为业务参数示例，可按实际需要扩展
	Parameter struct {
		TraveValue       int `json:"TraveValue"`
		TraveDuration    int `json:"TraveDuration"`
		TraveFrequency   int `json:"TraveFrequency"`
		PowerValue       int `json:"PowerValue"`
		PowerDuration    int `json:"PowerDuration"`
		PowerFrequency   int `json:"PowerFrequency"`
		VoltageValue     int `json:"VoltageValue"`
		VoltageDuration  int `json:"VoltageDuration"`
		VoltageFrequency int `json:"VoltageFrequency"`
	} `json:"Parameter"`
}

// LoadConfig 从给定路径读取并解析 JSON 配置，
// 如果文件不存在或解析失败，会返回错误。
// 如果未指定 SerialID/Baudrate，则使用默认值 "/dev/ttyPS1", 115200。
func LoadConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("无法读取配置文件 %s: %w", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
	}
	// 如果用户未指定串口，则使用默认
	if cfg.SerialID == "" {
		cfg.SerialID = "/dev/ttyPS1"
	}
	// 如果未指定波特率，则使用默认
	if cfg.Baudrate == 0 {
		cfg.Baudrate = 115200
	}
	return &cfg, nil
}
