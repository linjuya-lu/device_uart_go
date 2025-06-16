// 文件：rs485/protocol.go
package rs485

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"
)

// -------------------- IEC-101 常量定义 --------------------

const (
	IEC101_STABLE_BEGIN   = 0x10
	IEC101_VARIABLE_BEGIN = 0x68
	IEC101_END            = 0x16

	IEC101_68_LEN        = 15
	IEC101_FILE_LEN      = 11
	IEC101_CP56TIME_LEN  = 7
	IEC101_MAX_DATA_LEN  = 256
	IEC101_MAX_FILE_DATA = 220
)

// 报文类型 TI
const (
	IEC101_INIT_ENDS      = 0x46 // 初始化结束
	IEC101_TOTAL_CALL     = 0x64 // 总召唤
	IEC101_CLOCK_SYNC     = 0x67 // 时钟同步
	IEC101_TEST           = 0x68 // 链路测试
	IEC101_RESET          = 0x69 // 复位进程
	IEC101_FILE_CALL      = 0xD2 // 文件传输
	IEC101_REQ_LINK_STATE = 0x09 // 请求链路状态
	IEC101_FOLLOWED_LINK  = 0x0B // 响应链路状态
)

// COT (传送原因)
const (
	IEC101_COT_INIT_COMPLETE = 0x04 // 初始化完成
	IEC101_COT_REQ           = 0x05 // 请求/被请求
	IEC101_COT_CALL_ACTCON   = 0x07 // 激活确认
	IEC101_COT_CALL_ACTTERM  = 0x0A // 激活终止
)

// FC (功能码)
const (
	IEC101_RESET_LINK        = 0x00
	IEC101_TEST_LINK         = 0x02
	IEC101_USER_DATA         = 0x03
	IEC101_USER_NOREPLY_DATA = 0x04
	IEC101_FOLLOWED_RECO     = 0x00
	IEC101_FOLLOWED_NORECO   = 0x01
)

// 文件服务类型
const (
	IEC101_DIR_READ_ACT       = 0x01 // 读目录
	IEC101_DIR_READ_ACTION    = 0x02 // 读目录确认
	IEC101_FILE_READ_ACT      = 0x03 // 读文件激活
	IEC101_FILE_READ_ACTION   = 0x04 // 读文件激活确认
	IEC101_FILE_READ_DATA     = 0x05 // 读文件数据
	IEC101_FILE_READ_DATA_RES = 0x06 // 读文件数据响应
)

const IEC101_MAX_RCD_FILE_NUM = 2048

// -------------------- “短帧” 处理：ShortProcessPack --------------------

// ShortProcessPack 处理一个长度为 6 的固定帧 pkt。
// 格式： begin(1B=0x10) + ctrl(1B) + addr(2B) + cs(1B) + end(1B=0x16)
func (a *Agent) ShortProcessPack(pkt []byte) error {
	if len(pkt) != 6 {
		return fmt.Errorf("ShortPack 长度不是 6: %d", len(pkt))
	}
	// 校验头尾
	if pkt[0] != IEC101_STABLE_BEGIN || pkt[5] != IEC101_END {
		return errors.New("ShortPack 校验 起始/结束 字节失败")
	}
	// 校验和：pkt[4] == pkt[1]+pkt[2]+pkt[3]
	sum := byte(pkt[1] + pkt[2] + pkt[3])
	if pkt[4] != sum {
		return errors.New("ShortPack 校验和失败")
	}

	ctrl := pkt[1]
	dir := (ctrl >> 7) & 0x1
	prm := (ctrl >> 6) & 0x1
	funcCode := ctrl & 0x0F

	switch funcCode {
	case IEC101_RESET_LINK:
		if dir == 0 && prm == 1 {
			// 收到下行的“复位远方链路”命令
			log.Println("[Short] 收到 复位链路 命令")
			a.sendResetLinkAck()
			a.sendResetProcess()
			atomic.StoreUint32((*uint32)(&a.xEventFlag), uint32(IEC101_INIT_ENDS))
		} else if dir == 1 && prm == 0 {
			// 收到上行“复位链路 认可”
			event := byte(atomic.LoadUint32((*uint32)(&a.xEventFlag)))
			switch event {
			case IEC101_INIT_ENDS:
				log.Println("[Short] 链路建立，发送 可变帧 初始化结束")
				a.sendInitEnds()
				atomic.StoreUint32((*uint32)(&a.xEventFlag), 0)
			case IEC101_TOTAL_CALL:
				log.Println("[Short] 总召唤结束，发送 可变帧 总召终止")
				a.sendTotalCallTerm()
				atomic.StoreUint32((*uint32)(&a.xEventFlag), 0)
			case IEC101_FILE_CALL:
				log.Println("[Short] 文件服务阶段，继续发送 目录/数据")
				if a.fileDirFlag {
					a.sendDirResponse("/Emd/data/COMTRADE")
				}
				if a.fileReadFlag {
					a.fileReadDataTrans()
				}
				atomic.StoreUint32((*uint32)(&a.xEventFlag), 0)
			default:
				log.Printf("[Short] 未知事件标志: 0x%02X\n", event)
			}
		}
	case IEC101_TEST_LINK:
		// 心跳/链路测试，直接回 6B 认可 (FC=0)
		log.Println("[Short] 收到 心跳/链路测试，下行 6B 认可")
		a.sendHeartBeatAck()
	case IEC101_REQ_LINK_STATE:
		// 请求链路状态，回 FC=11 的 6B 帧
		log.Println("[Short] 收到 请求链路状态，发送 6B FC=11 响应")
		a.sendReqLinkStateResp()
	case IEC101_FOLLOWED_LINK:
		// 从动链路状态 (上行 fcb/fcv=1)，回复 FC=0(认可)
		log.Println("[Short] 收到 从动链路状态，发送 6B 认可")
		a.sendFollowedLinkAck()
	default:
		log.Printf("[Short] 未知 FC=0x%02X\n", funcCode)
	}
	return nil
}

// 发送 “复位链路 认可” 固定帧
func (a *Agent) sendResetLinkAck() {
	buf := make([]byte, 6)
	buf[0] = IEC101_STABLE_BEGIN
	// dir=1, prm=0, dfc/fcb/fcv=0, func=0 => 1000 0000 = 0x80
	buf[1] = 0x80
	binary.LittleEndian.PutUint16(buf[2:], 0x0001)
	buf[4] = buf[1] + buf[2] + buf[3]
	buf[5] = IEC101_END
	if err := a.SendPack(buf); err != nil {
		log.Printf("发送 复位链路 认可 失败: %v", err)
	}
}

// 发送 “复位进程” 固定帧 (dir=1, prm=1, func=0x09)
func (a *Agent) sendResetProcess() {
	buf := make([]byte, 6)
	buf[0] = IEC101_STABLE_BEGIN
	// dir=1 (bit7), prm=1 (bit6), func=9 (0x09)
	buf[1] = byte((1 << 7) | (1 << 6) | (0x09 & 0x0F))
	binary.LittleEndian.PutUint16(buf[2:], 0x0001)
	buf[4] = buf[1] + buf[2] + buf[3]
	buf[5] = IEC101_END
	if err := a.SendPack(buf); err != nil {
		log.Printf("发送 复位进程 失败: %v", err)
	}
}

// 发送 “心跳 认可” 固定帧 (dir=1, prm=0, func=0)
func (a *Agent) sendHeartBeatAck() {
	buf := make([]byte, 6)
	buf[0] = IEC101_STABLE_BEGIN
	buf[1] = 0x80 // dir=1, prm=0, func=0
	binary.LittleEndian.PutUint16(buf[2:], 0x0001)
	buf[4] = buf[1] + buf[2] + buf[3]
	buf[5] = IEC101_END
	if err := a.SendPack(buf); err != nil {
		log.Printf("发送 心跳 认可 失败: %v", err)
	}
}

// 发送 “请求链路状态 响应” 固定帧 (dir=1, prm=0, func=0x0B)
func (a *Agent) sendReqLinkStateResp() {
	buf := make([]byte, 6)
	buf[0] = IEC101_STABLE_BEGIN
	buf[1] = byte((1 << 7) | (0 << 6) | (0x0B & 0x0F)) // dir=1, func=11
	binary.LittleEndian.PutUint16(buf[2:], 0x0001)
	buf[4] = buf[1] + buf[2] + buf[3]
	buf[5] = IEC101_END
	if err := a.SendPack(buf); err != nil {
		log.Printf("发送 请求链路状态 响应 失败: %v", err)
	}
}

// 发送 “从动链路状态 认可” 固定帧 (dir=1, prm=1, func=0)
func (a *Agent) sendFollowedLinkAck() {
	buf := make([]byte, 6)
	buf[0] = IEC101_STABLE_BEGIN
	// dir=1, prm=1, func=0
	buf[1] = byte((1 << 7) | (1 << 6) | (0x00 & 0x0F))
	binary.LittleEndian.PutUint16(buf[2:], 0x0001)
	buf[4] = buf[1] + buf[2] + buf[3]
	buf[5] = IEC101_END
	if err := a.SendPack(buf); err != nil {
		log.Printf("发送 从动链路状态 失败: %v", err)
	}
}

// -------------------- “长帧” 处理：ProcessPack --------------------

// ProcessPack 负责处理可变帧 pkt：校验“68 LL LL 68”头、“CS”、“16”尾，
// 然后根据 TI 分发到不同业务函数。
func (a *Agent) ProcessPack(pkt []byte) error {
	if len(pkt) < 7 {
		return fmt.Errorf("可变帧长度 %d < 最小 7", len(pkt))
	}
	// 校验头
	if pkt[0] != IEC101_VARIABLE_BEGIN || pkt[3] != IEC101_VARIABLE_BEGIN {
		return errors.New("可变帧 起始 字节 错误")
	}
	// 校验尾
	if pkt[len(pkt)-1] != IEC101_END {
		return errors.New("可变帧 尾部 0x16 错误")
	}
	// 校验和：从 offset=4 累加到 len(pkt)-2，每字节累加
	var cs byte
	for i := 4; i < len(pkt)-2; i++ {
		cs += pkt[i]
	}
	if pkt[len(pkt)-2] != cs {
		return errors.New("可变帧 校验和 错误")
	}

	ti := pkt[7]
	cot := binary.LittleEndian.Uint16(pkt[9:11])
	data := pkt[15 : len(pkt)-2]

	switch ti {
	case IEC101_INIT_ENDS:
		log.Println("[Long] 收到 初始化结束")
		ti := pkt[7]              // ti 类型是 byte
		a.xEventFlag = uint32(ti) // 将 byte → uint32

	case IEC101_TOTAL_CALL:
		log.Println("[Long] 收到 总召唤")
		a.handleTotalCall(data)

	case IEC101_CLOCK_SYNC:
		log.Printf("[Long] 收到 时钟同步 (COT=0x%02X)", cot)
		a.handleClockSync(cot, data)

	case IEC101_TEST:
		log.Println("[Long] 收到 链路测试")
		a.handleLinkTest(data)

	case IEC101_RESET:
		log.Println("[Long] 收到 复位进程")
		a.handleResetCall(data)

	case IEC101_FILE_CALL:
		log.Println("[Long] 收到 文件传输")
		a.handleFileCall(data)

	default:
		log.Printf("[Long] 未知 TI=0x%02X\n", ti)
	}
	return nil
}

// -------------------- 业务分发与应答 --------------------

// sendInitEnds 发送“初始化结束”可变帧
func (a *Agent) sendInitEnds() {
	length := IEC101_68_LEN - 4 + 1 // data+1
	total := IEC101_68_LEN + 1 + 2  // 15+1+2 = 18
	buf := make([]byte, total)

	buf[0] = IEC101_VARIABLE_BEGIN
	buf[1] = byte(length)
	buf[2] = byte(length)
	buf[3] = IEC101_VARIABLE_BEGIN

	// ctrl.up: dir=1, prm=1(激活结束), func=0x03(用户数据)
	buf[4] = byte((1 << 7) | (1 << 6) | (0 << 5) | (0 << 4) | (0x03 & 0x0F))
	binary.LittleEndian.PutUint16(buf[5:], 0x0001)

	buf[7] = IEC101_INIT_ENDS
	buf[8] = 0x01 // VSQ

	binary.LittleEndian.PutUint16(buf[9:], IEC101_COT_INIT_COMPLETE)
	binary.LittleEndian.PutUint16(buf[11:], 0x0001)
	binary.LittleEndian.PutUint16(buf[13:], 0x0000)

	// data[0] = 0x02 (初始化完成)
	buf[15] = 0x02

	// 校验和 (offset 4~16)
	var cs byte
	for i := 4; i < 16; i++ {
		cs += buf[i]
	}
	buf[16] = cs
	buf[17] = IEC101_END

	if err := a.SendPack(buf); err != nil {
		log.Printf("发送 初始化结束 可变帧 失败: %v", err)
	}
}

// sendTotalCallTerm 发送“总召终止”可变帧
func (a *Agent) sendTotalCallTerm() {
	length := IEC101_68_LEN - 4 + 1 // data+1
	total := IEC101_68_LEN + 1 + 2  // 18
	buf := make([]byte, total)

	buf[0] = IEC101_VARIABLE_BEGIN
	buf[1] = byte(length)
	buf[2] = byte(length)
	buf[3] = IEC101_VARIABLE_BEGIN

	// ctrl.down: dir=1, prm=1, fcb=1, fcv=1, func=0x03
	buf[4] = byte((1 << 7) | (1 << 6) | (1 << 5) | (1 << 4) | (0x03 & 0x0F))
	binary.LittleEndian.PutUint16(buf[5:], 0x0001)

	buf[7] = IEC101_TOTAL_CALL
	buf[8] = 0x01 // VSQ

	binary.LittleEndian.PutUint16(buf[9:], IEC101_COT_CALL_ACTTERM)
	binary.LittleEndian.PutUint16(buf[11:], 0x0001)
	binary.LittleEndian.PutUint16(buf[13:], 0x0000)

	// data[0] = 0x14 (终止码)
	buf[15] = 0x14

	// 校验和 (offset 4~16)
	var cs byte
	for i := 4; i < 16; i++ {
		cs += buf[i]
	}
	buf[16] = cs
	buf[17] = IEC101_END

	if err := a.SendPack(buf); err != nil {
		log.Printf("发送 总召终止 可变帧 失败: %v", err)
	}
}

// handleTotalCall 处理 “总召唤” 可变帧
func (a *Agent) handleTotalCall(data []byte) {
	if len(data) < 1 || data[0] != 0x14 {
		log.Printf("总召唤 QOI 错误: 0x%02X\n", data[0])
		return
	}
	atomic.StoreUint32((*uint32)(&a.xEventFlag), uint32(IEC101_TOTAL_CALL))
	a.replyConfirmCom()
	a.sendTotalCallTerm()
}

// handleClockSync 处理 “时钟同步” 可变帧
func (a *Agent) handleClockSync(cot uint16, data []byte) {
	atomic.StoreUint32((*uint32)(&a.xEventFlag), uint32(IEC101_CLOCK_SYNC))
	a.replyConfirmCom()

	if cot == 0x05 {
		// 读时钟：取系统时间，打 7 字节 CP56Time
		now := time.Now()
		ct := encodeCP56Time(now)
		a.sendClockSyncResp(ct, IEC101_COT_REQ)

	} else if cot == 0x06 {
		// 写时钟：把 data 解码成 time.Time，然后 settimeofday
		newt, err := decodeCP56Time(data)
		if err != nil {
			log.Printf("时钟同步 解码失败: %v\n", err)
			return
		}
		tv := syscall.Timeval{
			Sec:  newt.Unix(),
			Usec: int64(newt.Nanosecond() / 1000),
		}
		if err := syscall.Settimeofday(&tv); err != nil {
			log.Printf("Settimeofday 失败: %v\n", err)
		} else {
			ct := encodeCP56Time(newt)
			a.sendClockSyncResp(ct, IEC101_COT_CALL_ACTCON)
		}
	}
}

// replyConfirmCom 发送“激活确认”可变帧 (COT=0x07)
func (a *Agent) replyConfirmCom() {
	length := IEC101_68_LEN - 4 + 0
	total := IEC101_68_LEN + 1 + 2
	buf := make([]byte, total)

	buf[0] = IEC101_VARIABLE_BEGIN
	buf[1] = byte(length)
	buf[2] = byte(length)
	buf[3] = IEC101_VARIABLE_BEGIN

	// ctrl.up: dir=1, prm=0, func=0x03
	buf[4] = byte((1 << 7) | (0 << 6) | (0 << 5) | (0 << 4) | (0x03 & 0x0F))
	binary.LittleEndian.PutUint16(buf[5:], 0x0001)

	buf[7] = byte(a.xEventFlag)

	buf[8] = 0x01 // VSQ

	binary.LittleEndian.PutUint16(buf[9:], IEC101_COT_CALL_ACTCON)
	binary.LittleEndian.PutUint16(buf[11:], 0x0001)
	binary.LittleEndian.PutUint16(buf[13:], 0x0000)

	// 没有 data
	var cs byte
	for i := 4; i < 15; i++ {
		cs += buf[i]
	}
	buf[15] = cs
	buf[16] = IEC101_END

	if err := a.SendPack(buf); err != nil {
		log.Printf("发送 激活确认 失败: %v\n", err)
	}
}

// sendClockSyncResp 发送时钟同步响应 (data 长度 7)
func (a *Agent) sendClockSyncResp(ct []byte, cot uint16) {
	length := IEC101_68_LEN - 4 + 7
	total := IEC101_68_LEN + 7 + 2
	buf := make([]byte, total)

	buf[0] = IEC101_VARIABLE_BEGIN
	buf[1] = byte(length)
	buf[2] = byte(length)
	buf[3] = IEC101_VARIABLE_BEGIN

	buf[4] = byte((1 << 7) | (0 << 6) | (0 << 5) | (0 << 4) | (0x03 & 0x0F))
	binary.LittleEndian.PutUint16(buf[5:], 0x0001)

	buf[7] = IEC101_CLOCK_SYNC
	buf[8] = 0x01

	binary.LittleEndian.PutUint16(buf[9:], cot)
	binary.LittleEndian.PutUint16(buf[11:], 0x0001)
	binary.LittleEndian.PutUint16(buf[13:], 0x0000)

	copy(buf[15:], ct)

	var cs byte
	for i := 4; i < 15+7; i++ {
		cs += buf[i]
	}
	buf[15+7] = cs
	buf[15+7+1] = IEC101_END

	if err := a.SendPack(buf); err != nil {
		log.Printf("发送 时钟同步 响应 失败: %v\n", err)
	}
}

// handleLinkTest 收到“链路测试”可变帧后，回 data=0xAA55
func (a *Agent) handleLinkTest(data []byte) {
	atomic.StoreUint32((*uint32)(&a.xEventFlag), uint32(IEC101_TEST))
	a.replyConfirmCom()

	length := IEC101_68_LEN - 4 + 2
	total := IEC101_68_LEN + 2 + 2
	buf := make([]byte, total)

	buf[0] = IEC101_VARIABLE_BEGIN
	buf[1] = byte(length)
	buf[2] = byte(length)
	buf[3] = IEC101_VARIABLE_BEGIN

	buf[4] = byte((1 << 7) | (0 << 6) | (0 << 5) | (0 << 4) | (0x03 & 0x0F))
	binary.LittleEndian.PutUint16(buf[5:], 0x0001)

	buf[7] = IEC101_TEST
	buf[8] = 0x01

	binary.LittleEndian.PutUint16(buf[9:], IEC101_COT_CALL_ACTCON)
	binary.LittleEndian.PutUint16(buf[11:], 0x0001)
	binary.LittleEndian.PutUint16(buf[13:], 0x0000)

	buf[15] = 0xAA
	buf[16] = 0x55

	var cs byte
	for i := 4; i < 15+2; i++ {
		cs += buf[i]
	}
	buf[15+2] = cs
	buf[15+2+1] = IEC101_END

	if err := a.SendPack(buf); err != nil {
		log.Printf("发送 链路测试 响应 失败: %v\n", err)
	}
}

// handleResetCall 收到“复位进程”可变帧后，如果 data[0]==1，则发送激活确认
func (a *Agent) handleResetCall(data []byte) {
	atomic.StoreUint32((*uint32)(&a.xEventFlag), uint32(IEC101_RESET))
	if len(data) < 1 || data[0] != 0x01 {
		log.Printf("复位进程 请求 QRP 错误: 0x%02X\n", data[0])
		return
	}
	a.replyConfirmCom()
}

// handleFileCall 处理可变帧“文件传输”，根据 operate_logo 进一步分发
func (a *Agent) handleFileCall(data []byte) {
	atomic.StoreUint32((*uint32)(&a.xEventFlag), uint32(IEC101_FILE_CALL))
	a.replyConfirmCom()
	if len(data) < 2 {
		log.Println("[File] Payload 太短")
		return
	}
	// packetType := data[0] // 如果需要可以保留
	operateLogo := data[1]
	payload := data[2:]

	switch operateLogo {
	case IEC101_DIR_READ_ACT:
		log.Println("[File] 读目录 请求")
		a.fileDirCall(payload)

	case IEC101_FILE_READ_ACT:
		log.Println("[File] 读文件激活 请求")
		a.fileReadAct(payload)

	case IEC101_FILE_READ_DATA_RES:
		log.Println("[File] 文件数据 响应")
		a.fileReadDataCon()

	default:
		log.Printf("[File] 未知 operate_logo=0x%02X\n", operateLogo)
	}
}

// fileDirCall 处理“读目录”命令：解析目录名、时间范围，调用 readDirInfo
func (a *Agent) fileDirCall(payload []byte) {
	if len(payload) < 1 {
		log.Println("[File] fileDirCall: payload 长度不足")
		return
	}
	dirNameLen := int(payload[0])
	if len(payload) < 1+dirNameLen+1+IEC101_CP56TIME_LEN*2 {
		log.Println("[File] fileDirCall: payload 长度不足")
		return
	}
	dirName := string(payload[1 : 1+dirNameLen])
	callSign := payload[1+dirNameLen]
	startCP := payload[1+dirNameLen+1 : 1+dirNameLen+1+IEC101_CP56TIME_LEN]
	endCP := payload[1+dirNameLen+1+IEC101_CP56TIME_LEN : 1+dirNameLen+1+IEC101_CP56TIME_LEN*2]

	if dirName != "COMTRADE" {
		log.Printf("[File] 目录名不对: %s\n", dirName)
		return
	}
	startT, err := decodeCP56Time(startCP)
	if err != nil {
		log.Printf("[File] 解析 StartTime 失败: %v\n", err)
		return
	}
	endT, err := decodeCP56Time(endCP)
	if err != nil {
		log.Printf("[File] 解析 EndTime 失败: %v\n", err)
		return
	}
	log.Printf("[File] Callsign=0x%02X, Start=%v, End=%v\n", callSign, startT, endT)
	a.readDirInfo(callSign, startT, endT)
}

// readDirInfo 扫描指定目录下的 COMTRADE 文件并发送目录响应
func (a *Agent) readDirInfo(callSign byte, start, end time.Time) {
	baseDir := "/Emd/data/COMTRADE"
	list := make([]string, 0, IEC101_MAX_RCD_FILE_NUM)

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(info.Name())
		if ext == ".dat" || ext == ".cfg" {
			// 按约定从文件名中解析时间戳 (示例)
			t, err := parseTimeFromFilename(info.Name())
			if err != nil {
				return nil
			}
			if t.Before(start) || t.After(end) {
				return nil
			}
			list = append(list, info.Name())
			if len(list) >= IEC101_MAX_RCD_FILE_NUM {
				return errors.New("文件数量超出上限，提前终止扫描")
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("[File] 扫描目录 %s 错误: %v\n", baseDir, err)
	}
	a.localFileNames = list
	a.fileDirFlag = false
	a.dirPackOffsetReset(list)
	a.sendDirResponse(baseDir)
}

// dirPackOffsetReset 计算目录总包数及最后一包大小，然后置 fileDirFlag=true
func (a *Agent) dirPackOffsetReset(list []string) {
	n := len(list)
	if n == 0 {
		a.dirPackNum = 0
		a.dirPackEnd = 0
	} else {
		a.dirPackNum = n / 4
		if n%4 != 0 {
			a.dirPackNum++
			a.dirPackEnd = n % 4
		} else {
			a.dirPackEnd = 4
		}
	}
	if a.dirPackNum > 0 {
		a.fileDirFlag = true
		a.fileDirOffset = 0
	}
}

// sendDirResponse 发送第 fileDirOffset 包 目录信息
func (a *Agent) sendDirResponse(baseDir string) {
	if !a.fileDirFlag {
		return
	}
	idx := a.fileDirOffset
	// n := len(a.localFileNames)
	isLast := (idx == a.dirPackNum-1)
	count := 4
	if isLast && a.dirPackEnd != 0 {
		count = a.dirPackEnd
	}

	// 计算包长度：IEC101_68_LEN + 1 + 9 + count*47 + 2
	payloadLen := IEC101_68_LEN + 1 + 9 + count*47 + 2
	buf := make([]byte, payloadLen)

	buf[0] = IEC101_VARIABLE_BEGIN
	buf[1] = byte(IEC101_68_LEN - 4 + 1 + 9 + count*47)
	buf[2] = buf[1]
	buf[3] = IEC101_VARIABLE_BEGIN

	buf[4] = byte((1 << 7) | (1 << 6) | (0 << 5) | (0 << 4) | (0x03 & 0x0F))
	binary.LittleEndian.PutUint16(buf[5:], 0x0001)

	buf[7] = IEC101_FILE_CALL
	buf[8] = 0x01
	binary.LittleEndian.PutUint16(buf[9:], IEC101_COT_REQ)
	binary.LittleEndian.PutUint16(buf[11:], 0x0001)
	binary.LittleEndian.PutUint16(buf[13:], 0x0000)

	off := IEC101_68_LEN + 1
	buf[off] = 0x02                     // packet_type = Dir_Read_Action
	buf[off+1] = IEC101_DIR_READ_ACTION // operate_logo
	buf[off+2] = 0x00                   // result = 0：成功
	off += 3

	// 文件列表信息：每个文件 47字节 = 1B len + 34B name + 1B attr + 4B size + 7B CP56Time
	for j := 0; j < count; j++ {
		name := a.localFileNames[idx*4+j]
		nameLen := byte(len(name))
		buf[off] = nameLen
		copy(buf[off+1:], name)
		if nameLen < 34 {
			for k := int(nameLen) + 1; k < 1+34; k++ {
				buf[off+k] = 0x00
			}
		}
		off1 := off + 1 + 34
		info, err := os.Stat(filepath.Join(baseDir, name))
		var fsize uint32
		if err != nil {
			fsize = 0
		} else {
			fsize = uint32(info.Size())
		}
		buf[off1] = 0x00 // file attribute
		binary.LittleEndian.PutUint32(buf[off1+1:], fsize)

		t, err := parseTimeFromFilename(name)
		if err != nil {
			for k := 0; k < 7; k++ {
				buf[off1+5+k] = 0x00
			}
		} else {
			ct := encodeCP56Time(t)
			copy(buf[off1+5:], ct)
		}
		off += 47
	}

	// 计算校验和
	var cs byte
	for i := 4; i < off; i++ {
		cs += buf[i]
	}
	buf[off] = cs
	buf[off+1] = IEC101_END

	if err := a.SendPack(buf); err != nil {
		log.Printf("发送 目录包 %d 失败: %v\n", idx+1, err)
	}
	if isLast {
		a.fileDirFlag = false
	} else {
		a.fileDirOffset++
	}
}

// fileReadAct 处理“读文件激活”
// payload 格式：1B 文件名长度 + 文件名 (ASCII)
func (a *Agent) fileReadAct(payload []byte) {
	if len(payload) < 1 {
		return
	}
	nameLen := int(payload[0])
	if len(payload) < 1+nameLen {
		return
	}
	name := string(payload[1 : 1+nameLen])
	fullPath := filepath.Join("/Emd/data/COMTRADE", name)
	info, err := os.Stat(fullPath)
	var fsize uint32
	if err != nil {
		log.Printf("[File] 打开文件 %s 失败: %v\n", fullPath, err)
		fsize = 0
	} else {
		fsize = uint32(info.Size())
	}
	a.rcdFileName = fullPath

	// 构造“读文件激活确认”可变帧
	// length := IEC101_68_LEN - 4 + 1 + nameLen + 4
	total := IEC101_68_LEN + 1 + 1 + nameLen + 4 + 2
	buf := make([]byte, total)

	buf[0] = IEC101_VARIABLE_BEGIN
	buf[1] = byte(IEC101_68_LEN - 4 + 1 + nameLen + 4)
	buf[2] = buf[1]
	buf[3] = IEC101_VARIABLE_BEGIN

	buf[4] = byte((1 << 7) | (1 << 6) | (0 << 5) | (0 << 4) | (0x03 & 0x0F))
	binary.LittleEndian.PutUint16(buf[5:], 0x0001)

	buf[7] = IEC101_FILE_CALL
	buf[8] = 0x01
	binary.LittleEndian.PutUint16(buf[9:], IEC101_COT_CALL_ACTCON)
	binary.LittleEndian.PutUint16(buf[11:], 0x0001)
	binary.LittleEndian.PutUint16(buf[13:], 0x0000)

	off := IEC101_68_LEN + 1
	buf[off] = 0x02                      // packet_type
	buf[off+1] = IEC101_FILE_READ_ACTION // operate_logo
	if fsize == 0 {
		buf[off+2] = 0x01 // result=1: 失败
	} else {
		buf[off+2] = 0x00 // result=0: 成功
	}
	off += 3

	buf[off] = byte(nameLen)
	copy(buf[off+1:], name)
	off += 1 + nameLen

	binary.LittleEndian.PutUint32(buf[off:], fsize)
	off += 4

	// 校验和
	var cs byte
	for i := 4; i < off; i++ {
		cs += buf[i]
	}
	buf[off] = cs
	buf[off+1] = IEC101_END

	if fsize > 0 {
		a.fileReadFlag = true
		a.fileOffset = 0
	}
	if err := a.SendPack(buf); err != nil {
		log.Printf("[File] 发送 读文件激活确认 失败: %v\n", err)
	}
}

// fileReadDataTrans 逐包发送文件原始数据 (每包最多 220B)
func (a *Agent) fileReadDataTrans() {
	info, err := os.Stat(a.rcdFileName)
	if err != nil {
		log.Printf("[File] 获取文件 %s 信息失败: %v\n", a.rcdFileName, err)
		a.fileReadFlag = false
		return
	}
	fileSize := info.Size()
	packetNum := int((fileSize + IEC101_MAX_FILE_DATA - 1) / IEC101_MAX_FILE_DATA)

	isLast := (a.fileOffset == packetNum-1)
	var chunkSize int
	if isLast {
		chunkSize = int(fileSize) - a.fileOffset*IEC101_MAX_FILE_DATA
	} else {
		chunkSize = IEC101_MAX_FILE_DATA
	}

	f, err := os.Open(a.rcdFileName)
	if err != nil {
		log.Printf("[File] 打开文件 %s 失败: %v\n", a.rcdFileName, err)
		a.fileReadFlag = false
		return
	}
	defer f.Close()

	if _, err := f.Seek(int64(a.fileOffset*IEC101_MAX_FILE_DATA), 0); err != nil {
		log.Printf("[File] Seek 失败: %v\n", err)
		a.fileReadFlag = false
		return
	}

	data := make([]byte, chunkSize)
	if _, err := io.ReadFull(f, data); err != nil {
		log.Printf("[File] 读取 %d 字节 失败: %v\n", chunkSize, err)
		a.fileReadFlag = false
		return
	}

	length := IEC101_68_LEN - 4 + IEC101_FILE_LEN + chunkSize
	total := IEC101_68_LEN + IEC101_FILE_LEN + chunkSize + 2
	buf := make([]byte, total)

	buf[0] = IEC101_VARIABLE_BEGIN
	buf[1] = byte(length)
	buf[2] = buf[1]
	buf[3] = IEC101_VARIABLE_BEGIN

	buf[4] = byte((1 << 7) | (1 << 6) | (0 << 5) | (0 << 4) | (0x03 & 0x0F))
	binary.LittleEndian.PutUint16(buf[5:], 0x0001)

	buf[7] = IEC101_FILE_CALL
	buf[8] = 0x01
	binary.LittleEndian.PutUint16(buf[9:], IEC101_COT_REQ)
	binary.LittleEndian.PutUint16(buf[11:], 0x0001)
	binary.LittleEndian.PutUint16(buf[13:], 0x0000)

	off := IEC101_68_LEN + 1
	buf[off] = 0x02                    // packet_type
	buf[off+1] = IEC101_FILE_READ_DATA // operate_logo
	// file_id = 0
	binary.LittleEndian.PutUint32(buf[off+2:], 0x00000000)
	// offset
	binary.LittleEndian.PutUint32(buf[off+6:], uint32(a.fileOffset*IEC101_MAX_FILE_DATA))
	if isLast {
		buf[off+10] = 0x00 // follow = 0
	} else {
		buf[off+10] = 0x01 // follow = 1
	}
	off += IEC101_FILE_LEN

	copy(buf[off:], data)
	off += chunkSize

	var cs byte
	for i := 4; i < off; i++ {
		cs += buf[i]
	}
	buf[off] = cs
	buf[off+1] = IEC101_END

	if err := a.SendPack(buf); err != nil {
		log.Printf("[File] 发送 数据包 %d 失败: %v\n", a.fileOffset+1, err)
	}
	if isLast {
		a.fileReadFlag = false
	} else {
		a.fileOffset++
	}
}

// fileReadDataCon 上位机确认收到文件数据后，只打印日志
func (a *Agent) fileReadDataCon() {
	log.Println("[File] 上位机 已确认 文件数据")
}

// parseTimeFromFilename 从文件名中解析时间 (示例: PQA20230906_143000_001.dat)
// 根据实际项目需求调整解析逻辑
func parseTimeFromFilename(name string) (time.Time, error) {
	if len(name) < 26 {
		return time.Time{}, errors.New("文件名长度不足，无法解析时间")
	}
	year := 2000 + int(name[13]-'0')*10 + int(name[14]-'0')
	month := time.Month(int(name[15]-'0')*10 + int(name[16]-'0'))
	day := int(name[17]-'0')*10 + int(name[18]-'0')
	hour := int(name[20]-'0')*10 + int(name[21]-'0')
	minute := int(name[22]-'0')*10 + int(name[23]-'0')
	second := int(name[24]-'0')*10 + int(name[25]-'0')
	return time.Date(year, month, day, hour, minute, second, 0, time.Local), nil
}

// -------------------- CP56Time 编解码 --------------------

// encodeCP56Time 将 Go 的 time.Time 编码为 7 字节 CP56Time
func encodeCP56Time(t time.Time) []byte {
	year := t.Year() - 2000
	mon := int(t.Month())
	day := t.Day()
	week := int(t.Weekday())
	hour := t.Hour()
	min := t.Minute()
	ms := uint16(t.Second()*1000 + t.Nanosecond()/1e6)

	buf := make([]byte, 7)
	binary.LittleEndian.PutUint16(buf[0:], ms)         // 毫秒
	buf[2] = byte(min & 0x3F)                          // 分钟
	buf[3] = byte(hour & 0x1F)                         // 小时
	buf[4] = byte((day & 0x1F) | ((week & 0x07) << 5)) // 日 + 周
	buf[5] = byte(mon & 0x0F)                          // 月
	buf[6] = byte((year & 0x7F))                       // 年
	return buf
}

// decodeCP56Time 将 7 字节 CP56Time 解码为 Go 的 time.Time
func decodeCP56Time(b []byte) (time.Time, error) {
	if len(b) < 7 {
		return time.Time{}, errors.New("CP56Time 长度不足")
	}
	ms := int(binary.LittleEndian.Uint16(b[0:2]))
	min := int(b[2] & 0x3F)
	hour := int(b[3] & 0x1F)
	day := int(b[4] & 0x1F)
	// week := int((b[4] >> 5) & 0x07) // 如需校验星期，可取出
	mon := time.Month(int(b[5] & 0x0F))
	year := 2000 + int(b[6]&0x7F)
	sec := ms / 1000
	nsec := (ms % 1000) * 1_000_000
	return time.Date(year, mon, day, hour, min, sec, nsec, time.Local), nil
}

// -------------------- “主循环” 入口：StartIEC101Agent --------------------

// StartIEC101Agent 启动 IEC-101 代理，直到 ctx 取消。
// cfgPath 为 JSON 配置文件路径，初始化失败时直接返回错误。
// 启动成功后，如果串口挂掉，会自动重连并打印日志，不会退出。
func StartIEC101Agent(ctx context.Context, cfgPath string) error {
	// 1) 加载 JSON 配置
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("LoadConfig 失败: %w", err)
	}
	agent := NewAgent(cfg)

	// 2) 初始化 RS-485 GPIO (控制 DE/RE)
	if err := agent.initSerialCon(); err != nil {
		log.Printf("警告：GPIO 初始化失败: %v\n", err)
		// 如果确实没有 GPIO 硬件，也可忽略
	}

	// 3) 主循环：只要 ctx 未取消，就保持服务
	for {
		select {
		case <-ctx.Done():
			// 收到取消信号，优雅退出
			agent.closeSerial()
			if agent.gpioFD != nil {
				agent.gpioFD.Close()
				agent.gpioFD = nil
			}
			log.Println("IEC-101 Agent 已停止 (上下文取消)")
			return nil
		default:
			// 尝试打开串口
			if err := agent.initSerial(); err != nil {
				log.Printf("打开串口 %s 失败: %v，10秒后重试...\n", cfg.SerialID, err)
				time.Sleep(10 * time.Second)
				continue
			}
			atomic.StoreInt32(&agent.isRunning, 1)
			log.Printf("串口 %s 已打开 (波特率 %d)\n", cfg.SerialID, cfg.Baudrate)

			// 4) 内层循环：调用 RecvPack -> ShortProcessPack/ProcessPack
			for atomic.LoadInt32(&agent.isRunning) == 1 {
				pkt, err := agent.RecvPack()
				if err != nil {
					if err == io.EOF {
						log.Println("串口 EOF 或 已关闭，准备重连...")
						atomic.StoreInt32(&agent.isRunning, 0)
						break
					}
					log.Printf("RecvPack 错误: %v\n", err)
					time.Sleep(100 * time.Millisecond)
					continue
				}
				if len(pkt) == 0 {
					// ReadTimeout 超时，没数据
					continue
				}
				if len(pkt) == 6 {
					if err := agent.ShortProcessPack(pkt); err != nil {
						log.Printf("ShortProcessPack 错误: %v\n", err)
					}
				} else if len(pkt) > 6 && len(pkt) <= IEC101_MAX_DATA_LEN {
					if err := agent.ProcessPack(pkt); err != nil {
						log.Printf("ProcessPack 错误: %v\n", err)
					}
				} else {
					log.Printf("收到 非法 长度 帧: %d\n", len(pkt))
				}
			}

			// 5) 如果 isRunning=0 或 串口出错，进行重连
			agent.closeSerial()
			log.Println("串口关闭，10秒后重连...")
			time.Sleep(10 * time.Second)
		}
	}
}
