// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2025 YourCompany
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"fmt"

	"github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
)

// resourceBinary 实现了二进制资源的读写；
// 它会从内存 DB 里拿到上一次写入的二进制帧，或把新帧写入 DB。
type resourceBinary struct {
	db *DB
}

// NewResourceBinary 构造函数，传入你的 DB 实例
func NewResourceBinary(db *DB) *resourceBinary {
	return &resourceBinary{db: db}
}

// value 从 DB 中获取最新的二进制数据，并封装成 CommandValue 上报
func (rb *resourceBinary) value(deviceName, deviceResourceName string) (*models.CommandValue, error) {
	// 从内存表里拿二进制帧
	res, err := rb.db.GetResource(deviceName, deviceResourceName)
	if err != nil {
		return nil, err
	}
	// res.Value 是 []byte
	cv, err := models.NewCommandValue(deviceResourceName, common.ValueTypeBinary, res.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to create CommandValue: %w", err)
	}
	return cv, nil
}

// write 接收上层下发的 CommandValue，把它的二进制内容写入 DB
func (rb *resourceBinary) write(param *models.CommandValue, deviceName, deviceResourceName string) error {
	// 从 CommandValue 拿字节 slice
	bytesVal, err := param.BinaryValue()
	if err != nil {
		return fmt.Errorf("invalid binary write for %s: %w", deviceResourceName, err)
	}
	// 更新到 DB
	if err := rb.db.UpdateResourceValue(deviceName, deviceResourceName, bytesVal); err != nil {
		return fmt.Errorf("db update failed: %w", err)
	}
	return nil
}
