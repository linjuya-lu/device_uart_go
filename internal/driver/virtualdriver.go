package driver

import (
	"fmt"
	"sync"

	"github.com/edgexfoundry/device-sdk-go/v4/pkg/interfaces"
	dsModels "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
)

// SerialProxyDriver 实现了 ProtocolDriver 接口，用于 RS-485 到 MQTT 的代理
type SerialProxyDriver struct {
	sdk interfaces.DeviceServiceSDK
	lc  logger.LoggingClient
	db  *DB
	mu  sync.Mutex
}

// Initialize 在服务启动时由 SDK 调用
func (d *SerialProxyDriver) Initialize(sdk interfaces.DeviceServiceSDK) error {
	d.sdk = sdk
	d.lc = sdk.LoggingClient()
	d.db.Init()
	return nil
}

// Start 在 SDK 完成初始化后调用，可在这里启动后台 goroutine
func (d *SerialProxyDriver) Start() error {
	// 如果不需要异步任务，直接返回 nil 即可
	return nil
}

// HandleReadCommands 从内存 DB 读取二进制数据并返回 CommandValue
func (d *SerialProxyDriver) HandleReadCommands(
	deviceName string,
	protocols map[string]models.ProtocolProperties,
	reqs []dsModels.CommandRequest,
) ([]*dsModels.CommandValue, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	results := make([]*dsModels.CommandValue, len(reqs))
	for i, req := range reqs {
		res, err := d.db.GetResource(deviceName, req.DeviceResourceName)
		if err != nil {
			return nil, fmt.Errorf("读取设备 %s 资源 %s 失败: %w", deviceName, req.DeviceResourceName, err)
		}

		cv, err := dsModels.NewCommandValue(
			req.DeviceResourceName,
			common.ValueTypeBinary,
			res.Value,
		)
		if err != nil {
			return nil, fmt.Errorf("创建 CommandValue 失败: %w", err)
		}
		results[i] = cv
	}
	return results, nil
}

// HandleWriteCommands 接收二进制写请求，保存到 DB 并通过 MQTT 发布
func (d *SerialProxyDriver) HandleWriteCommands(
	deviceName string,
	protocols map[string]models.ProtocolProperties,
	reqs []dsModels.CommandRequest,
	params []*dsModels.CommandValue,
) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, param := range params {
		raw, err := param.BinaryValue()
		if err != nil {
			return fmt.Errorf("无效二进制写入 %s: %w", param.DeviceResourceName, err)
		}

		// 更新 DB
		if err := d.db.UpdateResourceValue(deviceName, param.DeviceResourceName, raw); err != nil {
			return fmt.Errorf("更新 DB 失败: %w", err)
		}

		// 发布 MQTT
		// topic := fmt.Sprintf("edgex/%s/%s/response", deviceName, param.DeviceResourceName)
		// if err := d.mqttCli.PublishSerialAgent(
		// 	topic,
		// 	"v4", // apiVersion
		// 	"",   // correlationID
		// 	"",   // requestID
		// 	0,    // errorCode
		// 	raw,
		// ); err != nil {
		// 	return fmt.Errorf("MQTT 发布失败: %w", err)
		// }
	}
	return nil
}

// Stop 在服务停止时调用
func (d *SerialProxyDriver) Stop(force bool) error {
	d.lc.Info("SerialProxyDriver 停止")
	return nil
}

// AddDevice 当新增设备时调用，可初始化 DB 记录
func (d *SerialProxyDriver) AddDevice(
	deviceName string,
	protocols map[string]models.ProtocolProperties,
	adminState models.AdminState,
) error {
	// d.db.EnsureDevice(deviceName)
	return nil
}

// UpdateDevice 当设备更新时调用
func (d *SerialProxyDriver) UpdateDevice(
	deviceName string,
	protocols map[string]models.ProtocolProperties,
	adminState models.AdminState,
) error {
	// 可在此刷新配置信息
	return nil
}

// RemoveDevice 当设备移除时调用
func (d *SerialProxyDriver) RemoveDevice(
	deviceName string,
	protocols map[string]models.ProtocolProperties,
) error {
	d.db.DeleteDevice(deviceName)
	return nil
}

// Discover 驱动不支持发现
func (d *SerialProxyDriver) Discover() error {
	return fmt.Errorf("不支持 Discover")
}

// ValidateDevice 驱动不做设备验证
func (d *SerialProxyDriver) ValidateDevice(device models.Device) error {
	return nil
}
