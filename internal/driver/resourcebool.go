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

// resourceBool 负责读写布尔类型的 DeviceResource
type resourceBool struct {
	db *DB
}

// NewResourceBool 构造函数，传入你的 DB 实例
func NewResourceBool(db *DB) *resourceBool {
	return &resourceBool{db: db}
}

// value 从 DB 中获取最新的二进制数据（ASCII "true"/"false"）,
// 转成 bool 并封装成 CommandValue 上报
func (rb *resourceBool) value(deviceName, deviceResourceName string) (*models.CommandValue, error) {
	// 从内存表里拿存储的 []byte
	res, err := rb.db.GetResource(deviceName, deviceResourceName)
	if err != nil {
		return nil, err
	}

	// 解析成 string 再 ParseBool
	strVal := string(res.Value)
	b, err := strconv.ParseBool(strVal)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bool from %q: %w", strVal, err)
	}

	// 封装成 CommandValue
	cv, err := models.NewCommandValue(deviceResourceName, common.ValueTypeBool, b)
	if err != nil {
		return nil, fmt.Errorf("creating CommandValue: %w", err)
	}
	return cv, nil
}

// write 接收上层下发的 CommandValue，把它的布尔值写入 DB（存为 ASCII）
func (rb *resourceBool) write(param *models.CommandValue, deviceName, deviceResourceName string) error {
	// 从 CommandValue 拿 bool
	b, err := param.BoolValue()
	if err != nil {
		return fmt.Errorf("invalid bool write for %s: %w", deviceResourceName, err)
	}

	// 转成 ASCII bytes 存到 DB
	valBytes := []byte(strconv.FormatBool(b))
	if err := rb.db.UpdateResourceValue(deviceName, deviceResourceName, valBytes); err != nil {
		return fmt.Errorf("db update failed: %w", err)
	}
	return nil
}
