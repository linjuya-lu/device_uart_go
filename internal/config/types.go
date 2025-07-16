package config

// 描述串口设备
type Port struct {
	Name      string `yaml:"name"`      // 逻辑名称
	Device    string `yaml:"device"`    // 串口设备节点
	Type      string `yaml:"type"`      // uart/rs485/rs232
	Baudrate  int    `yaml:"baudrate"`  // 波特率
	DEPin     int    `yaml:"dePin"`     // RS-485 DE/RE 控制 GPIO 编号
	TimeoutMs int    `yaml:"timeoutMs"` // 读操作超时（毫秒）
}

// 协议对应主题
type Protocol struct {
	ID            string
	RequestTopic  string // 下行主题
	ResponseTopic string // 上行主题
}

// 端口绑定协议
type Binding struct {
	PortName   string `yaml:"portName"`
	ProtocolID string `yaml:"protocolId"`
}

// SerialProxyConfig 汇总了 Ports、Protocols、Bindings 等
type SerialProxyConfig struct {
	Ports           []Port     `yaml:"Ports"`
	Protocols       []Protocol `yaml:"Protocols"`
	Bindings        []Binding  `yaml:"Bindings"`
	DefaultProtocol string     `yaml:"DefaultProtocol"`
}
