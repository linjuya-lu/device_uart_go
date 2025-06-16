package driver

import (
	"sync"

	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
)

// Value 存储原始字节，DataType 仍可用来标记外层如何解析（比如 "raw", "uint16", "float32" 等）
type Resource struct {
	Name     string // 资源名
	DataType string // 标记 payload 类型
	Value    []byte // 原始二进制数据
}

// DB 是一个简单的内存存储：DeviceName → ResourceName → Resource
type DB struct {
	mu    sync.RWMutex
	store map[string]map[string]Resource
}

// NewDB 返回一个新建但未初始化的 DB
func NewDB() *DB {
	return &DB{
		store: make(map[string]map[string]Resource),
	}
}

// Init 清空所有数据，准备使用
func (d *DB) Init() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.store = make(map[string]map[string]Resource)
}

// AddResource 为指定设备添加或更新一个资源
// valueBytes 可以是任何二进制数据
func (d *DB) AddResource(
	deviceName string,
	resourceName string,
	dataType string,
	valueBytes []byte,
) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.store[deviceName]; !ok {
		d.store[deviceName] = make(map[string]Resource)
	}
	d.store[deviceName][resourceName] = Resource{
		Name:     resourceName,
		DataType: dataType,
		Value:    append([]byte(nil), valueBytes...), // 复制一份，避免外部修改
	}
}

// GetResource 获取指定设备的某个资源的当前值和类型
func (d *DB) GetResource(
	deviceName string,
	resourceName string,
) (Resource, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	devMap, devOk := d.store[deviceName]
	if !devOk {
		return Resource{}, errors.NewCommonEdgeX(
			errors.KindEntityDoesNotExist,
			"device not found",
			nil,
		)
	}
	res, resOk := devMap[resourceName]
	if !resOk {
		return Resource{}, errors.NewCommonEdgeX(
			errors.KindEntityDoesNotExist,
			"resource not found",
			nil,
		)
	}
	// 返回时也复制一份 Value
	res.Value = append([]byte(nil), res.Value...)
	return res, nil
}

// UpdateResourceValue 只修改资源的 Value 字段（不上报类型变化）
func (d *DB) UpdateResourceValue(
	deviceName string,
	resourceName string,
	newValue []byte,
) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	devMap, devOk := d.store[deviceName]
	if !devOk {
		return errors.NewCommonEdgeX(
			errors.KindEntityDoesNotExist,
			"device not found",
			nil,
		)
	}
	res, resOk := devMap[resourceName]
	if !resOk {
		return errors.NewCommonEdgeX(
			errors.KindEntityDoesNotExist,
			"resource not found",
			nil,
		)
	}
	// 替换 Value，并复制一份
	res.Value = append([]byte(nil), newValue...)
	devMap[resourceName] = res
	return nil
}

// DeleteDevice 删除整个设备及其所有资源
func (d *DB) DeleteDevice(deviceName string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.store, deviceName)
}

// Close 清理底层存储（可选）
func (d *DB) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.store = nil
}
