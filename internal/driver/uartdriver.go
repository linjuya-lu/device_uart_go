// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2019-2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// Package driver provides an implementation of a ProtocolDriver interface.
package driver

import (
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/edgexfoundry/device-sdk-go/v4/pkg/interfaces"
	dsModels "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/linjuya-lu/device_uart_go/internal/mqttclient"
)

type UartlDriver struct {
	lc         logger.LoggingClient
	asyncCh    chan<- *dsModels.AsyncValues
	locker     sync.Mutex
	sdk        interfaces.DeviceServiceSDK
	mqttClient mqtt.Client
}

var once sync.Once
var driver *UartlDriver

func NewUartDeviceDriver() interfaces.ProtocolDriver {
	once.Do(func() {
		driver = new(UartlDriver)
	})
	return driver
}

func (d *UartlDriver) Initialize(sdk interfaces.DeviceServiceSDK) error {
	d.sdk = sdk
	d.lc = sdk.LoggingClient()
	d.asyncCh = sdk.AsyncValuesChannel()

	if err := initVirtualResourceTable(d); err != nil {
		return fmt.Errorf("failed to init virtual resource table: %w", err)
	}

	// —— 1. 初始化 MQTT 客户端 —— //
	brokerURL := "tcp://localhost:1883"
	clientID := "serial-proxy-client"

	client, err := mqttclient.NewClient(brokerURL, clientID)
	if err != nil {
		return fmt.Errorf("初始化 MQTT 客户端失败: %w", err)
	}
	d.mqttClient = client

	// —— 2. 初始化串口代理 —— //
	if err := InitializeSerialProxy("./res/configuration.yaml", client); err != nil {
		return fmt.Errorf("初始化串口代理失败: %w", err)
	}

	return nil
}

func (d *UartlDriver) Start() error {
	d.lc.Infof("串口代理已启动")
	return nil
}

func (d *UartlDriver) HandleReadCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []dsModels.CommandRequest) (res []*dsModels.CommandValue, err error) {
	d.locker.Lock()
	defer driver.locker.Unlock()

	res = make([]*dsModels.CommandValue, len(reqs))

	for _, req := range reqs {
		resName := req.DeviceResourceName
		// 构造 CommandValue
		cv := &dsModels.CommandValue{
			DeviceResourceName: "",
			Type:               "",
			Value:              "",
			Origin:             time.Now().UnixNano(),
			Tags:               map[string]string{},
		}
		res = append(res, cv)
		d.lc.Infof("读取值: %s.%s ", deviceName, resName)
	}

	return res, nil
}

func (d *UartlDriver) HandleWriteCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []dsModels.CommandRequest,
	params []*dsModels.CommandValue) error {
	d.locker.Lock()
	defer driver.locker.Unlock()

	for i, req := range reqs {
		resName := req.DeviceResourceName
		cv := params[i]

		value := cv.Value

		d.lc.Infof("写入值: %s.%s = %v", deviceName, resName, value)
	}
	return nil
}

func (d *UartlDriver) Stop(force bool) error {
	d.lc.Info("VirtualDriver.Stop: device-virtual driver is stopping...")

	return nil
}

func (d *UartlDriver) AddDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	d.lc.Debugf("a new Device is added: %s", deviceName)

	return nil
}

func (d *UartlDriver) UpdateDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	d.lc.Debugf("Device %s is updated", deviceName)
	return nil
}

func (d *UartlDriver) RemoveDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	d.lc.Debugf("Device %s is removed", deviceName)
	return nil
}

func initVirtualResourceTable(driver *UartlDriver) error {

	return nil
}

func (d *UartlDriver) Discover() error {
	return fmt.Errorf("driver's Discover function isn't implemented")
}

func (d *UartlDriver) ValidateDevice(device models.Device) error {
	d.lc.Debug("Driver's ValidateDevice function isn't implemented")
	return nil
}
