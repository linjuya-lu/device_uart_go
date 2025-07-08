package config

// Port 描述一个串口设备
type Port struct {
	Name      string `yaml:"name"`      // 逻辑名称
	Device    string `yaml:"device"`    // 串口设备节点
	Type      string `yaml:"type"`      // uart/rs485/rs232
	Baudrate  int    `yaml:"baudrate"`  // 波特率
	DEPin     int    `yaml:"dePin"`     // RS-485 DE/RE 控制 GPIO 编号
	TimeoutMs int    `yaml:"timeoutMs"` // 读操作超时（毫秒）
}

// 一种协议对应的 MQTT 主题
type Protocol struct {
	ID            string // 协议标识符
	RequestTopic  string // 下行主题
	ResponseTopic string // 上行主题
}

// 端口绑定使用哪种协议
type Binding struct {
	PortName   string `yaml:"portName"`   // Port.Name
	ProtocolID string `yaml:"protocolId"` // Protocol.ID
}

// SerialProxyConfig 汇总了 Ports、Protocols、Bindings 等
type SerialProxyConfig struct {
	Ports           []Port     `yaml:"Ports"`
	Protocols       []Protocol `yaml:"Protocols"`
	Bindings        []Binding  `yaml:"Bindings"`
	DefaultProtocol string     `yaml:"DefaultProtocol"`
}
