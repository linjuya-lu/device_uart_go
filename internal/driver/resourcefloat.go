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

// resourceFloat 负责读写浮点类型的 DeviceResource
type resourceFloat struct {
	db *DB
}

// NewResourceFloat 构造函数，传入你的 DB 实例
func NewResourceFloat(db *DB) *resourceFloat {
	return &resourceFloat{db: db}
}

// value 从 DB 中获取最新存储的 ASCII bytes，解析为 float32/float64，
// 并封装成 CommandValue 上报
func (rf *resourceFloat) value(
	deviceName, deviceResourceName, dataType string,
) (*models.CommandValue, error) {
	res, err := rf.db.GetResource(deviceName, deviceResourceName)
	if err != nil {
		return nil, err
	}

	// 把字节转成字符串，再根据 dataType 解析
	strVal := string(res.Value)
	var cv *models.CommandValue
	switch dataType {
	case common.ValueTypeFloat32:
		f, err := strconv.ParseFloat(strVal, 32)
		if err != nil {
			return nil, fmt.Errorf("parse float32 from %q: %w", strVal, err)
		}
		cv, err = models.NewCommandValue(deviceResourceName, common.ValueTypeFloat32, float32(f))
		if err != nil {
			return nil, fmt.Errorf("creating float32 CommandValue: %w", err)
		}
	case common.ValueTypeFloat64:
		f, err := strconv.ParseFloat(strVal, 64)
		if err != nil {
			return nil, fmt.Errorf("parse float64 from %q: %w", strVal, err)
		}
		cv, err = models.NewCommandValue(deviceResourceName, common.ValueTypeFloat64, f)
		if err != nil {
			return nil, fmt.Errorf("creating float64 CommandValue: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported float dataType: %s", dataType)
	}

	return cv, nil
}

// write 接收上层下发的 CommandValue，把浮点值存为 ASCII bytes 写入 DB
func (rf *resourceFloat) write(
	param *models.CommandValue,
	deviceName, deviceResourceName string,
) error {
	var (
		strVal string
		err    error
	)

	// 根据 CV 类型取值并格式化
	switch param.Type {
	case common.ValueTypeFloat32:
		var v float32
		if v, err = param.Float32Value(); err == nil {
			strVal = strconv.FormatFloat(float64(v), 'f', -1, 32)
		}
	case common.ValueTypeFloat64:
		var v float64
		if v, err = param.Float64Value(); err == nil {
			strVal = strconv.FormatFloat(v, 'f', -1, 64)
		}
	default:
		return fmt.Errorf("resourceFloat.write: unsupported type %s", param.Type)
	}

	if err != nil {
		return fmt.Errorf("invalid float write for %s: %w", deviceResourceName, err)
	}

	// 写入 DB（存为 ASCII bytes）
	if err := rf.db.UpdateResourceValue(deviceName, deviceResourceName, []byte(strVal)); err != nil {
		return fmt.Errorf("db update failed: %w", err)
	}
	return nil
}
