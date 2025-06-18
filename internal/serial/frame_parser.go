// internal/serial/frame_parser.go
package serial

import "bytes"

// FrameParser is a function that extracts exactly one complete frame from a byte stream.
// It returns:
//   - frame: the extracted complete frame (nil if not enough data yet)
//   - rest:  any leftover bytes after the frame (to be carried over into next parse)
//   - err:   any parsing error (in which case the entire buffer should be discarded)
type FrameParser func(buf []byte) (frame []byte, rest []byte, err error)

// Parsers maps your protocol IDs to their corresponding FrameParser.
var Parsers = map[string]FrameParser{
	"customProto23": parseProto23,
	"customProto16": parseProto16,
	"customProto55": parseProto55,
}

// parseProto23 looks for frames that start with 0xAA and end with 0x55.
func parseProto23(buf []byte) ([]byte, []byte, error) {
	// find header
	i := bytes.IndexByte(buf, 0xAA)
	if i < 0 {
		return nil, buf, nil
	}
	buf = buf[i:]
	// find tail
	j := bytes.IndexByte(buf, 0x55)
	if j < 0 {
		return nil, buf, nil
	}
	// extract frame and leftover
	return buf[:j+1], buf[j+1:], nil
}

// parseProto16 temporarily reuses parseProto23 logic.
// Replace with your actual logic for “customProto16”.
func parseProto16(buf []byte) ([]byte, []byte, error) {
	return parseProto23(buf)
}

// parseProto55 temporarily reuses parseProto23 logic.
// Replace with your actual logic for “customProto55”.
func parseProto55(buf []byte) ([]byte, []byte, error) {
	return parseProto23(buf)
}
