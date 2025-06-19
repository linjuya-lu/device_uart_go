// internal/serial/frame_parser.go
package serial

import "bytes"

// FrameParser 定义了一个从字节流中提取完整帧的函数类型。
// 它返回：
//   - frame: 抽取出的完整帧（若数据不足以组成完整帧则返回 nil）
//   - rest: 余下未处理的字节（用于下一次解析时继续累积）
//   - err:  解析出错时的错误（此时应丢弃整个缓冲区）
type FrameParser func(buf []byte) (frame []byte, rest []byte, err error)

// Parsers 将协议 ID 映射到对应的 FrameParser 实现。
var Parsers = map[string]FrameParser{
	"customProto23": parseProto23,
	"customProto16": parseProto16,
	"customProto55": parseProto55,
}

// parseProto23 从 buf 中查找以 0xAA 开头、0x55 结尾的帧。
func parseProto23(buf []byte) ([]byte, []byte, error) {
	// 查找帧头 0xAA
	i := bytes.IndexByte(buf, 0xAA)
	if i < 0 {
		// 缓冲区中无帧头，全部数据留待下次解析
		return nil, buf, nil
	}
	// 丢弃帧头前的杂散数据
	buf = buf[i:]
	// 查找帧尾 0x55
	j := bytes.IndexByte(buf, 0x55)
	if j < 0 {
		// 尚未找到帧尾，保留全部数据
		return nil, buf, nil
	}
	// 提取完整帧，并返回剩余数据
	return buf[:j+1], buf[j+1:], nil
}

// parseProto16 暂时复用 parseProto23 逻辑，
// 后续可根据 customProto16 协议格式替换实现。
func parseProto16(buf []byte) ([]byte, []byte, error) {
	return parseProto23(buf)
}

// parseProto55 暂时复用 parseProto23 逻辑，
// 后续可根据 customProto55 协议格式替换实现。
func parseProto55(buf []byte) ([]byte, []byte, error) {
	return parseProto23(buf)
}
