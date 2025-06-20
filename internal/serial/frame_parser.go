package serial

import (
	"bytes"
	"fmt"
)

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
// func parseProto23(buf []byte) ([]byte, []byte, error) {
// 	// 查找帧头 0xAA
// 	i := bytes.IndexByte(buf, 0xAA)
// 	if i < 0 {
// 		// 缓冲区中无帧头，全部数据留待下次解析
// 		return nil, buf, nil
// 	}
// 	// 丢弃帧头前的杂散数据
// 	buf = buf[i:]
// 	// 查找帧尾 0x55
// 	j := bytes.IndexByte(buf, 0x55)
// 	if j < 0 {
// 		// 尚未找到帧尾，保留全部数据
// 		return nil, buf, nil
// 	}
// 	// 提取完整帧，并返回剩余数据
// 	return buf[:j+1], buf[j+1:], nil
// }

func parseProto23(buf []byte) ([]byte, []byte, error) {
	// 1. 打印接收到的原始 buf
	fmt.Printf(">>> parseProto23 passthrough, buf len=%d, buf=% X\n", len(buf), buf)

	// 2. 如果 buf 为空，就返回 nil，继续等数据
	if len(buf) == 0 {
		return nil, buf, nil
	}

	// 3. 把整个 buf 当作一帧 frame 返回，rest 置空
	frame := buf
	rest := []byte{}

	// 4. 调试打印一下我们要发的 frame
	fmt.Printf("    passthrough frame len=%d, frame=% X\n", len(frame), frame)

	return frame, rest, nil
}

func parseProto16(buf []byte) ([]byte, []byte, error) {
	// 查找帧头 0x16
	i := bytes.IndexByte(buf, 0x16)
	if i < 0 {
		// 缓冲区中无帧头，全部数据留待下次解析
		return nil, buf, nil
	}
	// 丢弃帧头前的杂散数据
	buf = buf[i:]
	// 查找帧尾 0xBB
	j := bytes.IndexByte(buf, 0x33)
	if j < 0 {
		// 尚未找到帧尾，保留全部数据
		return nil, buf, nil
	}
	// 提取完整帧，并返回剩余数据
	return buf[:j+1], buf[j+1:], nil
}

func parseProto55(buf []byte) ([]byte, []byte, error) {
	// 查找帧头 0x55
	i := bytes.IndexByte(buf, 0x55)
	if i < 0 {
		// 缓冲区中无帧头，全部数据留待下次解析
		return nil, buf, nil
	}
	// 丢弃帧头前的杂散数据
	buf = buf[i:]
	// 查找帧尾 0xCC
	j := bytes.IndexByte(buf, 0xCC)
	if j < 0 {
		// 尚未找到帧尾，保留全部数据
		return nil, buf, nil
	}
	// 提取完整帧，并返回剩余数据
	return buf[:j+1], buf[j+1:], nil
}
