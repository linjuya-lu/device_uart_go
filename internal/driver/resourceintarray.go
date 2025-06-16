// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2025 YourCompany
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"encoding/json"
	"fmt"

	"github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
)

// resourceIntArray 负责读写整型数组类型的 DeviceResource
type resourceIntArray struct {
	db *DB
}

// NewResourceIntArray 构造函数，传入你的 DB 实例
func NewResourceIntArray(db *DB) *resourceIntArray {
	return &resourceIntArray{db: db}
}

// value 从 DB 中获取最新存储的 JSON bytes，解析为对应位宽的切片,
// 并封装成 CommandValue 上报
func (ri *resourceIntArray) value(
	deviceName, deviceResourceName, dataType string,
) (*models.CommandValue, error) {
	res, err := ri.db.GetResource(deviceName, deviceResourceName)
	if err != nil {
		return nil, err
	}

	switch dataType {
	case common.ValueTypeInt8Array:
		var arr []int8
		if err := json.Unmarshal(res.Value, &arr); err != nil {
			return nil, fmt.Errorf("unmarshal int8 array: %w", err)
		}
		return models.NewCommandValue(deviceResourceName, common.ValueTypeInt8Array, arr)
	case common.ValueTypeInt16Array:
		var arr []int16
		if err := json.Unmarshal(res.Value, &arr); err != nil {
			return nil, fmt.Errorf("unmarshal int16 array: %w", err)
		}
		return models.NewCommandValue(deviceResourceName, common.ValueTypeInt16Array, arr)
	case common.ValueTypeInt32Array:
		var arr []int32
		if err := json.Unmarshal(res.Value, &arr); err != nil {
			return nil, fmt.Errorf("unmarshal int32 array: %w", err)
		}
		return models.NewCommandValue(deviceResourceName, common.ValueTypeInt32Array, arr)
	case common.ValueTypeInt64Array:
		var arr []int64
		if err := json.Unmarshal(res.Value, &arr); err != nil {
			return nil, fmt.Errorf("unmarshal int64 array: %w", err)
		}
		return models.NewCommandValue(deviceResourceName, common.ValueTypeInt64Array, arr)
	default:
		return nil, fmt.Errorf("unsupported integer-array dataType: %s", dataType)
	}
}

// write 接收上层下发的 CommandValue，把整数数组写入 DB（以 JSON bytes 存储）
func (ri *resourceIntArray) write(
	param *models.CommandValue,
	deviceName, deviceResourceName string,
) error {
	var (
		raw []byte
		err error
	)
	switch param.Type {
	case common.ValueTypeInt8Array:
		var arr []int8
		if arr, err = param.Int8ArrayValue(); err == nil {
			raw, err = json.Marshal(arr)
		}
	case common.ValueTypeInt16Array:
		var arr []int16
		if arr, err = param.Int16ArrayValue(); err == nil {
			raw, err = json.Marshal(arr)
		}
	case common.ValueTypeInt32Array:
		var arr []int32
		if arr, err = param.Int32ArrayValue(); err == nil {
			raw, err = json.Marshal(arr)
		}
	case common.ValueTypeInt64Array:
		var arr []int64
		if arr, err = param.Int64ArrayValue(); err == nil {
			raw, err = json.Marshal(arr)
		}
	default:
		return fmt.Errorf("resourceIntArray.write: unsupported type %s", param.Type)
	}

	if err != nil {
		return fmt.Errorf("invalid integer-array write for %s: %w", deviceResourceName, err)
	}

	if err := ri.db.UpdateResourceValue(deviceName, deviceResourceName, raw); err != nil {
		return fmt.Errorf("db update failed: %w", err)
	}
	return nil
}
