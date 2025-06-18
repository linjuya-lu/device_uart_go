// internal/serial/serial.go

package serial

import (
	"fmt"
	"time"

	"github.com/linjuya-lu/device_uart_go/internal/config"
)

// Port 是整个 serial 包对外暴露的通用串口接口，支持原始字节与帧级操作
type Port interface {
	// 打开串口（含 GPIO 初始化）
	Open() error
	// 关闭串口（含 GPIO 释放）
	Close() error
	// Read 读取原始字节，实现 io.Reader
	Read(p []byte) (int, error)
	// Write 写入原始字节，实现 io.Writer
	Write(p []byte) (int, error)
	// Name 返回逻辑端口名称
	Name() string

	// ReadFrame 从串口里按协议取出一整帧（固定帧/可变帧）数据
	ReadFrame() ([]byte, error)
	// WriteFrame 直接向串口写入一整帧数据
	WriteFrame(frame []byte) error
}

// NewPort 根据配置创建对应的串口实现（UART / RS-485 / RS-232）
func NewPort(cfg config.Port) (Port, error) {
	switch cfg.Type {
	case "uart":
		return NewUARTPort(cfg), nil
	case "rs485":
		return NewRS485Port(cfg), nil
	case "rs232":
		return NewRS232Port(cfg), nil
	default:
		return nil, fmt.Errorf("unknown port type %s", cfg.Type)
	}
}

// StartReadLoop 会在后台协程里不断从 p.ReadFrame() 读取完整帧，
// 每拿到一帧就调用 onFrame(portName, data)
func StartReadLoop(p Port, onFrame func(portName string, data []byte)) {
	go func() {
		for {
			frame, err := p.ReadFrame()
			if err != nil {
				// 读取出错，稍后重试
				time.Sleep(time.Millisecond * 100)
				continue
			}
			onFrame(p.Name(), frame)
		}
	}()
}
