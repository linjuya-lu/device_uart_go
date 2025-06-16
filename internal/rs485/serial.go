package rs485

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/tarm/serial"
)

// Agent 结构体声明（和 Config 在同一个包里）
type Agent struct {
	cfg       *Config
	port      *serial.Port
	gpioFD    *os.File
	isRunning int32

	// 以下为协议处理和文件传输所需的状态字段
	xEventFlag     uint32
	fileDirFlag    bool
	fileDirOffset  int
	dirPackNum     int
	dirPackEnd     int
	fileReadFlag   bool
	rcdFileName    string
	fileOffset     int
	localFileNames []string
}

// NewAgent 返回一个新的 Agent 实例
func NewAgent(cfg *Config) *Agent {
	return &Agent{
		cfg:            cfg,
		localFileNames: make([]string, 0, IEC101_MAX_FILE_DATA),
	}
}

// -------------------- GPIO 部分 (RS-485 DE/RE 控制) --------------------

// exportGPIO 如果尚未导出对应 GPIO，则写入 /sys/class/gpio/export
func exportGPIO(num int) error {
	path := "/sys/class/gpio/export"
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("无法打开 %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(fmt.Sprintf("%d", num)); err != nil {
		// 如果已经导出，会返回 “device or resource busy”，可忽略
		if !os.IsExist(err) {
			return fmt.Errorf("向 %s 写入 GPIO %d 失败: %w", path, num, err)
		}
	}
	return nil
}

// setGPIODirection 设置 GPIO 方向 ("in" 或 "out")
func setGPIODirection(num int, dir string) error {
	path := fmt.Sprintf("/sys/class/gpio/gpio%d/direction", num)
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("无法打开 %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(dir); err != nil {
		return fmt.Errorf("向 %s 写入方向 %s 失败: %w", path, dir, err)
	}
	return nil
}

// openGPIOValue 打开 GPIO 的 /sys/class/gpio/gpioN/value
func openGPIOValue(num int) (*os.File, error) {
	path := fmt.Sprintf("/sys/class/gpio/gpio%d/value", num)
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("无法打开 %s: %w", path, err)
	}
	return f, nil
}

// initSerialCon 初始化 RS-485 控制 GPIO（假设 GPIO 914），并将其拉低（进入接收模式）
func (a *Agent) initSerialCon() error {
	const gpioNum = 914
	if err := exportGPIO(gpioNum); err != nil {
		log.Printf("警告：GPIO %d 导出失败 (可能已导出)：%v", gpioNum, err)
	}
	// 等待 sysfs 节点生成
	time.Sleep(100 * time.Millisecond)

	if err := setGPIODirection(gpioNum, "out"); err != nil {
		return fmt.Errorf("设置 GPIO %d 方向失败: %w", gpioNum, err)
	}

	fd, err := openGPIOValue(gpioNum)
	if err != nil {
		return fmt.Errorf("打开 GPIO %d value 失败: %w", gpioNum, err)
	}
	// 默认拉低，进入“接收”模式
	if _, err := fd.WriteString("0"); err != nil {
		fd.Close()
		return fmt.Errorf("向 GPIO %d 写 '0' 失败: %w", gpioNum, err)
	}
	a.gpioFD = fd
	return nil
}

// setDERE 切换 RS-485 DE/RE；val=0 表示“接收”，val=1 表示“发送”
func (a *Agent) setDERE(val byte) error {
	if a.gpioFD == nil {
		return errors.New("GPIO 未初始化")
	}
	s := "0"
	if val != 0 {
		s = "1"
	}
	if _, err := a.gpioFD.WriteString(s); err != nil {
		return fmt.Errorf("写 GPIO DE/RE %s 失败: %w", s, err)
	}
	return nil
}

// -------------------- 串口初始化 / 关闭 / 读一帧 --------------------

// initSerial 打开并配置串口，使用 github.com/tarm/serial
func (a *Agent) initSerial() error {
	c := &serial.Config{
		Name:        a.cfg.SerialID,
		Baud:        a.cfg.Baudrate,
		ReadTimeout: 500 * time.Millisecond,
	}
	port, err := serial.OpenPort(c)
	if err != nil {
		return fmt.Errorf("打开串口 %s 失败: %w", a.cfg.SerialID, err)
	}
	a.port = port
	return nil
}

// closeSerial 关闭串口
func (a *Agent) closeSerial() {
	if a.port != nil {
		a.port.Close()
		a.port = nil
	}
}

// RecvPack 从串口读取一整帧 IEC-101 数据，最多 256 字节
// - Peek 一字节判断是 0x10(固定帧) 还是 0x68(可变帧)；
// - 如果都不是，则丢弃一个字节后返回错误，以便上层重试。
// - 固定帧长度固定 6，直接 ReadFull 6 个字节。
// - 可变帧先 ReadFull(4)，取出 len，再 ReadFull(len+2)。
func (a *Agent) RecvPack() ([]byte, error) {
	reader := bufio.NewReader(a.port)

	// 1. Peek 一字节判断帧头
	hdr, err := reader.Peek(1)
	if err != nil {
		return nil, err
	}
	switch hdr[0] {
	case IEC101_STABLE_BEGIN:
		// 固定帧：6 字节
		buf := make([]byte, 6)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return nil, err
		}
		return buf, nil

	case IEC101_VARIABLE_BEGIN:
		// 可变帧：先读 4 字节 (68 LL LL 68)
		header := make([]byte, 4)
		if _, err := io.ReadFull(reader, header); err != nil {
			return nil, err
		}
		length := int(header[1]) // len1 = len2，一般取 header[1]
		total := 4 + length + 2  // 4(头) + length 数据区 + 2(校验+尾)
		buf := make([]byte, total)
		copy(buf[:4], header)
		if _, err := io.ReadFull(reader, buf[4:]); err != nil {
			return nil, err
		}
		return buf, nil

	default:
		// 既不是 0x10，也不是 0x68，可能有“脏数据”，先丢弃一个字节再返回错误
		_, _ = reader.ReadByte()
		return nil, fmt.Errorf("非法起始字节 0x%02X，已丢弃一个字节", hdr[0])
	}
}

// SendPack 发送一帧 IEC-101 数据：
// - 先切到“发送”模式 (GPIO 写 1)，延时几毫秒；
// - 再写串口；等待足够时间让整帧发送完；
// - 最后切回“接收”模式 (GPIO 写 0)。
func (a *Agent) SendPack(b []byte) error {
	// 切到“发送”模式
	if err := a.setDERE(1); err != nil {
		return fmt.Errorf("切换到发送模式失败: %w", err)
	}
	// 等几毫秒，确保 GPIO 已生效
	time.Sleep(5 * time.Millisecond)

	n, err := a.port.Write(b)
	if err != nil {
		a.setDERE(0)
		return fmt.Errorf("向串口写入失败: %w", err)
	}
	// 计算等待时间：每帧 10 个比特 (1 字节 8 比特 + 起止位共 10 比特)
	delay := time.Duration(n) * time.Second * 10 / time.Duration(a.cfg.Baudrate)
	time.Sleep(delay)

	// 切回“接收”模式
	if err := a.setDERE(0); err != nil {
		return fmt.Errorf("切换到接收模式失败: %w", err)
	}
	return nil
}
