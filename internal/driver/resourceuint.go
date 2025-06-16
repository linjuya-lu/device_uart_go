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

// resourceUint 负责读写无符号整型 DeviceResource
type resourceUint struct {
	db *DB
}

// NewResourceUint 构造函数，传入你的 DB 实例
func NewResourceUint(db *DB) *resourceUint {
	return &resourceUint{db: db}
}

// value 从 DB 中获取最新存储的 ASCII bytes，解析为对应位宽的无符号整数，
// 并封装成 CommandValue 上报
func (ru *resourceUint) value(
	deviceName, deviceResourceName, dataType string,
) (*models.CommandValue, error) {
	res, err := ru.db.GetResource(deviceName, deviceResourceName)
	if err != nil {
		return nil, err
	}

	strVal := string(res.Value)
	var cv *models.CommandValue

	switch dataType {
	case common.ValueTypeUint8:
		v, err := strconv.ParseUint(strVal, 10, 8)
		if err != nil {
			return nil, fmt.Errorf("parse uint8 from %q: %w", strVal, err)
		}
		cv, err = models.NewCommandValue(deviceResourceName, common.ValueTypeUint8, uint8(v))
	case common.ValueTypeUint16:
		v, err := strconv.ParseUint(strVal, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("parse uint16 from %q: %w", strVal, err)
		}
		cv, err = models.NewCommandValue(deviceResourceName, common.ValueTypeUint16, uint16(v))
	case common.ValueTypeUint32:
		v, err := strconv.ParseUint(strVal, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parse uint32 from %q: %w", strVal, err)
		}
		cv, err = models.NewCommandValue(deviceResourceName, common.ValueTypeUint32, uint32(v))
	case common.ValueTypeUint64:
		v, err := strconv.ParseUint(strVal, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse uint64 from %q: %w", strVal, err)
		}
		cv, err = models.NewCommandValue(deviceResourceName, common.ValueTypeUint64, v)
	default:
		return nil, fmt.Errorf("unsupported unsigned integer dataType: %s", dataType)
	}

	if err != nil {
		return nil, fmt.Errorf("creating Uint CommandValue: %w", err)
	}

	return cv, nil
}

// write 接收上层下发的 CommandValue，把无符号整数值转成 ASCII bytes 写入 DB
func (ru *resourceUint) write(
	param *models.CommandValue,
	deviceName, deviceResourceName string,
) error {
	var strVal string
	var err error

	switch param.Type {
	case common.ValueTypeUint8:
		var v uint8
		if v, err = param.Uint8Value(); err == nil {
			strVal = strconv.FormatUint(uint64(v), 10)
		}
	case common.ValueTypeUint16:
		var v uint16
		if v, err = param.Uint16Value(); err == nil {
			strVal = strconv.FormatUint(uint64(v), 10)
		}
	case common.ValueTypeUint32:
		var v uint32
		if v, err = param.Uint32Value(); err == nil {
			strVal = strconv.FormatUint(uint64(v), 10)
		}
	case common.ValueTypeUint64:
		var v uint64
		if v, err = param.Uint64Value(); err == nil {
			strVal = strconv.FormatUint(v, 10)
		}
	default:
		return fmt.Errorf("resourceUint.write: unsupported type %s", param.Type)
	}

	if err != nil {
		return fmt.Errorf("invalid uint write for %s: %w", deviceResourceName, err)
	}

	// 写入 DB（存为 ASCII bytes）
	if err := ru.db.UpdateResourceValue(deviceName, deviceResourceName, []byte(strVal)); err != nil {
		return fmt.Errorf("db update failed: %w", err)
	}
	return nil
}
