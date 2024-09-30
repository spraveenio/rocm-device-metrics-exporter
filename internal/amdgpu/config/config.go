//
// Copyright(C) Advanced Micro Devices, Inc. All rights reserved.
//
// You may not use this software and documentation (if any) (collectively,
// the "Materials") except in compliance with the terms and conditions of
// the Software License Agreement included with the Materials or otherwise as
// set forth in writing and signed by you and an authorized signatory of AMD.
// If you do not have a copy of the Software License Agreement, contact your
// AMD representative for a copy.
//
// You agree that you will not reverse engineer or decompile the Materials,
// in whole or in part, except as allowed by applicable law.
//
// THE MATERIALS ARE DISTRIBUTED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OR
// REPRESENTATIONS OF ANY KIND, EITHER EXPRESS OR IMPLIED.
//

package config

import (
	"os"
	"strconv"

	"github.com/pensando/device-metrics-exporter/internal/amdgpu/globals"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/logger"
)

type Config struct {
	serverPort        uint32
	metricsConfigPath string
}

func NewConfig(mPath string) *Config {
	c := &Config{
		serverPort:        globals.AMDListenPort,
		metricsConfigPath: mPath,
	}
	logger.Log.Printf("Running Config :%+v", mPath)
	return c
}

func (c *Config) SetServerPort(port uint32) error {
	logger.Log.Printf("Server reconfigured from config file to %v", port)
	c.serverPort = port
	return nil
}

func (c *Config) GetServerPort() uint32 {
	if os.Getenv("METRICS_EXPORTER_PORT") != "" {
		logger.Log.Printf("METRICS_EXPORTER_PORT env set, override serport")
		portStr := os.Getenv("METRICS_EXPORTER_PORT")
		number, err := strconv.Atoi(portStr)
		if err != nil {
			return c.serverPort
		}
		return uint32(number)
	}
	return c.serverPort
}

func (c *Config) GetMetricsConfigPath() string {
	return c.metricsConfigPath
}
