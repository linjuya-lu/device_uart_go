package serial

import (
	"fmt"
	"time"

	"github.com/linjuya-lu/device_uart_go/internal/config"
	"github.com/tarm/serial"
)

type UARTPort struct {
	cfg    config.Port
	handle *serial.Port
}

func NewUARTPort(cfg config.Port) Port {
	return &UARTPort{cfg: cfg}
}

func (u *UARTPort) Open() error {
	sc := &serial.Config{
		Name:        u.cfg.Device,
		Baud:        u.cfg.Baudrate,
		ReadTimeout: time.Duration(u.cfg.TimeoutMs) * time.Millisecond,
	}
	p, err := serial.OpenPort(sc)
	if err != nil {
		return fmt.Errorf("open UART %s failed: %w", u.cfg.Device, err)
	}
	u.handle = p
	return nil
}

func (u *UARTPort) Close() error {
	if u.handle != nil {
		return u.handle.Close()
	}
	return nil
}

func (u *UARTPort) Read(p []byte) (int, error) {
	return u.handle.Read(p)
}

func (u *UARTPort) Write(p []byte) (int, error) {
	fmt.Printf("⇨ UARTPort.Write writing %d bytes: % X (as string: %q)\n", len(p), p, string(p))

	n, err := u.handle.Write(p)
	if err != nil {
		return n, fmt.Errorf("UART write failed: %w", err)
	}
	return n, nil
}

// Name 返回逻辑名称
func (u *UARTPort) Name() string {
	return u.cfg.Name
}

// ReadFrame 按最简单策略把一次 Read 当作一帧返回
// （也可以在这里做更复杂的“找头找尾”逻辑）
func (u *UARTPort) ReadFrame() ([]byte, error) {
	buf := make([]byte, 4096)
	n, err := u.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (u *UARTPort) WriteFrame(frame []byte) error {
	// 打印即将写入的数据：十六进制和字符串两种格式
	fmt.Printf("⇨ UARTPort.WriteFrame writing %d bytes: % X (as string: %q)\n", len(frame), frame, string(frame))

	// 写入串口
	if _, err := u.Write(frame); err != nil {
		return fmt.Errorf("UART WriteFrame failed: %w", err)
	}
	return nil
}
