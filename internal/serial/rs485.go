package serial

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"github.com/linjuya-lu/device_uart_go/internal/config"
	"github.com/tarm/serial"
)

// RS485Port 实现了 RS-485 半双工物理层的帧级读写
// - Open/Close 管理串口和 GPIO
// - Read/Write 提供原始字节接口
// - ReadFrame/WriteFrame 提供按帧读写接口

type RS485Port struct {
	cfg    config.Port  // 端口配置
	port   *serial.Port // 串口句柄
	gpioFD *os.File     // DE/RE 控制 GPIO 节点
	buf    []byte       // 缓存用于帧级解析
}

// 构造 RS485Port 实例
func NewRS485Port(cfg config.Port) Port {
	return &RS485Port{cfg: cfg, buf: make([]byte, 0)}
}

// Open 导出 GPIO 并打开串口
func (r *RS485Port) Open() error {
	// 导出 GPIO
	if err := exportGPIO(r.cfg.DEPin); err != nil {
		return fmt.Errorf("export GPIO %d failed: %w", r.cfg.DEPin, err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := setGPIODirection(r.cfg.DEPin, "out"); err != nil {
		return fmt.Errorf("set GPIO %d direction: %w", r.cfg.DEPin, err)
	}
	f, err := openGPIOValue(r.cfg.DEPin)
	if err != nil {
		return fmt.Errorf("open GPIO %d value: %w", r.cfg.DEPin, err)
	}
	// 默认低电平 (接收)
	if _, err := f.WriteString("0"); err != nil {
		f.Close()
		return fmt.Errorf("init GPIO %d low: %w", r.cfg.DEPin, err)
	}
	r.gpioFD = f

	// 打开串口
	serCfg := &serial.Config{
		Name:        r.cfg.Device,
		Baud:        r.cfg.Baudrate,
		ReadTimeout: time.Duration(r.cfg.TimeoutMs) * time.Millisecond,
	}
	p, err := serial.OpenPort(serCfg)
	if err != nil {
		r.gpioFD.Close()
		return fmt.Errorf("open serial %s failed: %w", r.cfg.Device, err)
	}
	r.port = p
	return nil
}

// Close 关闭串口和 GPIO
func (r *RS485Port) Close() error {
	var firstErr error
	if r.port != nil {
		if err := r.port.Close(); err != nil {
			firstErr = err
		}
	}
	if r.gpioFD != nil {
		if err := r.gpioFD.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Read 实现 io.Reader
func (r *RS485Port) Read(p []byte) (int, error) {
	return r.port.Read(p)
}

// Write 实现 io.Writer，注意并不会自动切换 DE/RE
func (r *RS485Port) Write(p []byte) (int, error) {
	return r.port.Write(p)
}

// ReadFrame 按 IEC101 (0x68…0x16) 协议从缓存+串口中提取完整帧
func (r *RS485Port) ReadFrame() ([]byte, error) {
	// 读入新数据
	tmp := make([]byte, 256)
	n, err := r.port.Read(tmp)
	if err != nil {
		return nil, err
	}
	r.buf = append(r.buf, tmp[:n]...)

	// 搜索帧头 (0x68)
	head := bytes.IndexByte(r.buf, 0x68)
	if head < 0 {
		// 无帧头，丢弃脏数据
		r.buf = nil
		return nil, nil
	}
	if len(r.buf) < head+4 {
		// 数据不足以解析长度
		return nil, nil
	}
	length := int(r.buf[head+1])
	total := head + 4 + length + 2 // 4-byte header + data + checksum + tail
	if len(r.buf) < total {
		// 未读完一帧
		return nil, nil
	}
	frame := make([]byte, total-head)
	copy(frame, r.buf[head:total])
	// 更新缓存
	r.buf = r.buf[total:]
	return frame, nil
}

// WriteFrame 切到发送 → 写整帧 → 切回接收
func (r *RS485Port) WriteFrame(frame []byte) error {
	// 切到发送
	if _, err := r.gpioFD.WriteString("1"); err != nil {
		return fmt.Errorf("GPIO DE high failed: %w", err)
	}
	time.Sleep(5 * time.Millisecond)

	n, err := r.port.Write(frame)
	if err != nil {
		// 出错切回接收
		r.gpioFD.WriteString("0")
		return fmt.Errorf("serial write failed: %w", err)
	}
	// 等待所有比特发出 (10 bits/byte)
	time.Sleep(time.Duration(n*10) * time.Second / time.Duration(r.cfg.Baudrate))

	// 切回接收
	if _, err := r.gpioFD.WriteString("0"); err != nil {
		return fmt.Errorf("GPIO DE low failed: %w", err)
	}
	return nil
}

// Name 返回端口名称
func (r *RS485Port) Name() string {
	return r.cfg.Name
}

// -------- GPIO 辅助函数 --------
func exportGPIO(pin int) error {
	f, err := os.OpenFile("/sys/class/gpio/export", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, _ = f.WriteString(fmt.Sprint(pin)) // 若已导出则忽略错误
	return nil
}

func setGPIODirection(pin int, dir string) error {
	path := fmt.Sprintf("/sys/class/gpio/gpio%d/direction", pin)
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(dir)
	return err
}

func openGPIOValue(pin int) (*os.File, error) {
	path := fmt.Sprintf("/sys/class/gpio/gpio%d/value", pin)
	return os.OpenFile(path, os.O_RDWR, 0)
}
