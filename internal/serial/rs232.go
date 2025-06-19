package serial

import (
	"bufio"
	"fmt"
	"io"
	"time"

	"github.com/linjuya-lu/device_uart_go/internal/config"
	aliasserial "github.com/tarm/serial"
)

// RS232Port 实现了标准 RS-232 全双工串口的帧级收发：
// - Open/Close 管理串口
// - Read/Write 提供原始字节接口
// - ReadFrame/WriteFrame 支持按帧自动解析/发送

type RS232Port struct {
	cfg  config.Port
	port *aliasserial.Port
}

// NewRS232Port 构造 RS232Port
func NewRS232Port(cfg config.Port) Port {
	return &RS232Port{cfg: cfg}
}

// Open 打开并配置串口
func (r *RS232Port) Open() error {
	sc := &aliasserial.Config{
		Name:        r.cfg.Device,
		Baud:        r.cfg.Baudrate,
		ReadTimeout: time.Duration(r.cfg.TimeoutMs) * time.Millisecond,
	}
	p, err := aliasserial.OpenPort(sc)
	if err != nil {
		return fmt.Errorf("open serial %s failed: %w", r.cfg.Device, err)
	}
	r.port = p
	return nil
}

// Close 关闭串口
func (r *RS232Port) Close() error {
	if r.port != nil {
		return r.port.Close()
	}
	return nil
}

// Read 读取原始字节，实现 io.Reader
func (r *RS232Port) Read(p []byte) (int, error) {
	return r.port.Read(p)
}

// Write 写入原始字节，实现 io.Writer
func (r *RS232Port) Write(p []byte) (int, error) {
	n, err := r.port.Write(p)
	if err != nil {
		return n, fmt.Errorf("serial write failed: %w", err)
	}
	return n, nil
}

// Name 返回逻辑名称
func (r *RS232Port) Name() string {
	return r.cfg.Name
}

// ReadFrame 按自定义协议自动组帧读取：
// 固定帧头 0x10 (6 字节)
// 可变帧头 0x68 (先 4 字节 header，再读 length+2)
func (r *RS232Port) ReadFrame() ([]byte, error) {
	reader := bufio.NewReader(r.port)
	hdr, err := reader.Peek(1)
	if err != nil {
		return nil, err
	}
	switch hdr[0] {
	case 0x10:
		// 固定帧，6 字节
		buf := make([]byte, 6)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return nil, err
		}
		return buf, nil

	case 0x68:
		// 可变帧，先读 4 字节头
		head := make([]byte, 4)
		if _, err := io.ReadFull(reader, head); err != nil {
			return nil, err
		}
		length := int(head[1])
		total := 4 + length + 2
		buf := make([]byte, total)
		copy(buf[:4], head)
		if _, err := io.ReadFull(reader, buf[4:]); err != nil {
			return nil, err
		}
		return buf, nil

	default:
		// 丢弃一个字节后重试
		reader.ReadByte()
		return nil, fmt.Errorf("invalid start byte 0x%02X dropped", hdr[0])
	}
}

// WriteFrame 直接写整帧数据
func (r *RS232Port) WriteFrame(frame []byte) error {
	if _, err := r.port.Write(frame); err != nil {
		return fmt.Errorf("serial write frame failed: %w", err)
	}
	return nil
}
