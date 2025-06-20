// Package config 定义了串口代理服务的配置结构
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

// Protocol 描述一种协议对应的 MQTT 主题
type Protocol struct {
	ID            string `yaml:"id"`            // 协议标识符，对应配置中的 key
	RequestTopic  string `yaml:"requestTopic"`  // 下发命令主题
	ResponseTopic string `yaml:"responseTopic"` // 上报数据主题
}

// Binding 描述某个端口绑定使用哪种协议
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

// func main() {
//     // 加载配置（只执行一次）
//     if err := config.LoadConfig("res/configuration.yaml"); err != nil {
//         log.Fatalf("加载配置失败：%v", err)
//     }

//     // 获取全局配置
//     sp := config.SerialCfg

//     // 打开并初始化所有串口
//     for _, port := range sp.Ports {
//         initSerialPort(port) // 根据 port.Type/port.Device 配置
//     }

//     // 运行主逻辑…
// }
