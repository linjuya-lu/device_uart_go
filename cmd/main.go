// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018-2022 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/edgexfoundry/device-sdk-go/v4/pkg/startup"

	device_virtual "github.com/edgexfoundry/device-virtual-go"
	"github.com/linjuya-lu/device_uart_go/internal/driver"
)

const (
	serviceName string = "device-virtual"
)

func main() {
	d := driver.NewVirtualDeviceDriver()
	startup.Bootstrap(serviceName, device_virtual.Version, d)
}
