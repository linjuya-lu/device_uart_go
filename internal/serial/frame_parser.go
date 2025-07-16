package serial

import (
	"encoding/hex"
	"fmt"
	"strings"
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

// parseProto23: ASCII 流 buf 中查找以 "AA" 开头、"55" 结尾的帧（每个字符代表一个十六进制字符）
func parseProto23(buf []byte) ([]byte, []byte, error) {
	s := string(buf)
	fmt.Printf("parseProto23 ▶ in ascii: %s\n", s)
	// 查找 "AA" 头
	start := strings.Index(s, "AA")
	if start < 0 {
		return nil, buf, nil
	}
	// 在头之后查找 "55" 尾
	idx := strings.Index(s[start+2:], "55")
	if idx < 0 {
		return nil, buf, nil
	}
	end := start + 2 + idx + 2 // 包含尾标长度
	frameHex := s[start:end]
	restAscii := s[end:]
	fmt.Printf("parseProto23 ▶ frameHex: %s, restAscii: %s\n", frameHex, restAscii)

	// 将十六进制字符串 decode 为字节
	frame, err := hex.DecodeString(frameHex)
	if err != nil {
		return nil, buf, fmt.Errorf("parseProto23 decode error: %w", err)
	}
	return frame, []byte(restAscii), nil
}

// parseProto16: ASCII 流 buf 中查找以 "16" 开头、"33" 结尾的帧
func parseProto16(buf []byte) ([]byte, []byte, error) {
	s := string(buf)
	start := strings.Index(s, "16")
	if start < 0 {
		return nil, buf, nil
	}
	idx := strings.Index(s[start+2:], "33")
	if idx < 0 {
		return nil, buf, nil
	}
	end := start + 2 + idx + 2
	frameHex := s[start:end]
	restAscii := s[end:]
	frame, err := hex.DecodeString(frameHex)
	if err != nil {
		return nil, buf, fmt.Errorf("parseProto16 decode error: %w", err)
	}
	return frame, []byte(restAscii), nil
}

// parseProto55: ASCII 流 buf 中查找以 "55" 开头、"CC" 结尾的帧
func parseProto55(buf []byte) ([]byte, []byte, error) {
	s := string(buf)
	start := strings.Index(s, "55")
	if start < 0 {
		return nil, buf, nil
	}
	idx := strings.Index(s[start+2:], "CC")
	if idx < 0 {
		return nil, buf, nil
	}
	end := start + 2 + idx + 2
	frameHex := s[start:end]
	restAscii := s[end:]
	frame, err := hex.DecodeString(frameHex)
	if err != nil {
		return nil, buf, fmt.Errorf("parseProto55 decode error: %w", err)
	}
	return frame, []byte(restAscii), nil
}
