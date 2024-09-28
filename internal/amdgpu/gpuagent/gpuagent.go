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
	"context"
	"fmt"
	"sync"

	"github.com/pensando/device-metrics-exporter/internal/amdgpu/gen/amdgpu"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/logger"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/metricsutil"
	"google.golang.org/grpc"
)

const (
	gpuagentAddr = "0.0.0.0:50061"
)

type GPUAgentClient struct {
	conn   *grpc.ClientConn
	mh     *metricsutil.MetricsHandler
	client amdgpu.GPUSvcClient
	m      *metrics // client specific metrics
	sync.Mutex
}

func NewAgent(mh *metricsutil.MetricsHandler) (*GPUAgentClient, error) {
	conn, err := grpc.Dial(gpuagentAddr, grpc.WithInsecure())
	if err != nil {
		logger.Log.Printf("err :%v", err)
		return nil, err
	}
	client := amdgpu.NewGPUSvcClient(conn)
	ag := &GPUAgentClient{
		conn:   conn,
		client: client,
		mh:     mh,
	}
	mh.RegisterMetricsClient(ag)
	ag.InitConfigs()
	err = ag.UpdateStaticMetrics()
	if err != nil {
		return nil, err
	}
	return ag, nil

}

func (ga *GPUAgentClient) getMetrics() (*amdgpu.GPUGetResponse, error) {
	ga.Lock()
	defer ga.Unlock()
	if ga.client == nil {
		return nil, fmt.Errorf("client closed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &amdgpu.GPUGetRequest{}
	res, err := ga.client.GPUGet(ctx, req)
	return res, err
}

func (ga *GPUAgentClient) Close() {
	ga.Lock()
	defer ga.Unlock()
	if ga.conn != nil {
		ga.conn.Close()
		ga.client = nil
	}
}

func (ga *GPUAgentClient) UpdateMetricsStats() error {
	// send the req to gpuclient
	res, err := ga.getMetrics()
	if err != nil {
		logger.Log.Printf("err :%v", err)
		return err
	}
	if res != nil && res.ApiStatus != 0 {
		logger.Log.Printf("resp status :%v", res.ApiStatus)
		return fmt.Errorf("%v", res.ApiStatus)
	}
	ga.updateGPUToMetrics(res)
	return nil
}
