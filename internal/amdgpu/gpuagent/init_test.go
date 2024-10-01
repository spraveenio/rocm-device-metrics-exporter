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

package gpuagent

import (
	"fmt"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/config"
	amdgpu "github.com/pensando/device-metrics-exporter/internal/amdgpu/gen/amdgpu"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/logger"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/metricsutil"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/mock_gen"
	"gotest.tools/assert"
)

var (
	mock_resp *amdgpu.GPUGetResponse
	mockCtl   *gomock.Controller
	gpuMockCl *mock_gen.MockGPUSvcClient
	mh        *metricsutil.MetricsHandler
	mConfig   *config.Config
)

func setupTest(t *testing.T) func(t *testing.T) {
	t.Logf("============= TestSetup %v ===============", t.Name())

	fmt.Println("LOGDIR", os.Getenv("LOGDIR"))

	logger.Init()

	mockCtl = gomock.NewController(t)

	gpuMockCl = mock_gen.NewMockGPUSvcClient(mockCtl)

	mock_resp = &amdgpu.GPUGetResponse{
		ApiStatus: amdgpu.ApiStatus_API_STATUS_OK,
		Response: []*amdgpu.GPU{
			{
				Spec: &amdgpu.GPUSpec{
					Id: []byte(uuid.New().String()),
				},
				Status: &amdgpu.GPUStatus{
					SerialNum: "mock-serial",
				},
				Stats: &amdgpu.GPUStats{
					FanSpeed: 1.1,
				},
			},
			{
				Spec: &amdgpu.GPUSpec{
					Id: []byte(uuid.New().String()),
				},
				Status: &amdgpu.GPUStatus{
					SerialNum: "mock-serial-2",
				},
				Stats: &amdgpu.GPUStats{
					FanSpeed: 2.1,
				},
			},
		},
	}

	gomock.InOrder(
		gpuMockCl.EXPECT().GPUGet(gomock.Any(), gomock.Any()).Return(mock_resp, nil).AnyTimes(),
	)

	mConfig = config.NewConfig("config.json")

	mh, _ = metricsutil.NewMetrics(mConfig)
	mh.InitConfig()

	return func(t *testing.T) {
		t.Logf("============= Test:TearDown %v ===============", t.Name())
		mockCtl.Finish()
	}
}

func getNewAgent(t *testing.T) *GPUAgentClient {
	ga, err := NewAgent(mh)
	assert.Assert(t, err == nil, "error creating new agent : %v", err)
	ga.client = gpuMockCl
	return ga
}
