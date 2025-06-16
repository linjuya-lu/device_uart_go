// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2025 YourCompany
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"fmt"
	"strconv"

	"github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
)

// resourceInt 负责读写整型 DeviceResource
type resourceInt struct {
	db *DB
}

// NewResourceInt 构造函数，传入你的 DB 实例
func NewResourceInt(db *DB) *resourceInt {
	return &resourceInt{db: db}
}

// value 从 DB 中获取最新存储的 ASCII bytes，解析为对应位宽的整数，
// 并封装成 CommandValue 上报
func (ri *resourceInt) value(
	deviceName, deviceResourceName, dataType string,
) (*models.CommandValue, error) {
	res, err := ri.db.GetResource(deviceName, deviceResourceName)
	if err != nil {
		return nil, err
	}

	strVal := string(res.Value)
	var cv *models.CommandValue

	switch dataType {
	case common.ValueTypeInt8:
		v, err := strconv.ParseInt(strVal, 10, 8)
		if err != nil {
			return nil, fmt.Errorf("parse int8 from %q: %w", strVal, err)
		}
		cv, err = models.NewCommandValue(deviceResourceName, common.ValueTypeInt8, int8(v))
	case common.ValueTypeInt16:
		v, err := strconv.ParseInt(strVal, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("parse int16 from %q: %w", strVal, err)
		}
		cv, err = models.NewCommandValue(deviceResourceName, common.ValueTypeInt16, int16(v))
	case common.ValueTypeInt32:
		v, err := strconv.ParseInt(strVal, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parse int32 from %q: %w", strVal, err)
		}
		cv, err = models.NewCommandValue(deviceResourceName, common.ValueTypeInt32, int32(v))
	case common.ValueTypeInt64:
		v, err := strconv.ParseInt(strVal, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse int64 from %q: %w", strVal, err)
		}
		cv, err = models.NewCommandValue(deviceResourceName, common.ValueTypeInt64, v)
	default:
		return nil, fmt.Errorf("unsupported integer dataType: %s", dataType)
	}

	if err != nil {
		return nil, fmt.Errorf("creating Integer CommandValue: %w", err)
	}

	return cv, nil
}

// write 接收上层下发的 CommandValue，把整数值转成 ASCII bytes 写入 DB
func (ri *resourceInt) write(
	param *models.CommandValue,
	deviceName, deviceResourceName string,
) error {
	var strVal string
	var err error

	switch param.Type {
	case common.ValueTypeInt8:
		var v int8
		if v, err = param.Int8Value(); err == nil {
			strVal = strconv.FormatInt(int64(v), 10)
		}
	case common.ValueTypeInt16:
		var v int16
		if v, err = param.Int16Value(); err == nil {
			strVal = strconv.FormatInt(int64(v), 10)
		}
	case common.ValueTypeInt32:
		var v int32
		if v, err = param.Int32Value(); err == nil {
			strVal = strconv.FormatInt(int64(v), 10)
		}
	case common.ValueTypeInt64:
		var v int64
		if v, err = param.Int64Value(); err == nil {
			strVal = strconv.FormatInt(v, 10)
		}
	default:
		return fmt.Errorf("resourceInt.write: unsupported type %s", param.Type)
	}

	if err != nil {
		return fmt.Errorf("invalid integer write for %s: %w", deviceResourceName, err)
	}

	// 写入 DB（存为 ASCII bytes）
	if err := ri.db.UpdateResourceValue(deviceName, deviceResourceName, []byte(strVal)); err != nil {
		return fmt.Errorf("db update failed: %w", err)
	}
	return nil
}
