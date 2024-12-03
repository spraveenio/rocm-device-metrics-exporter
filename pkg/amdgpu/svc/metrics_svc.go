/**
# Copyright (c) Advanced Micro Devices, Inc. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the \"License\");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an \"AS IS\" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package metricsserver

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/gen/metricssvc"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/logger"
	"google.golang.org/protobuf/types/known/emptypb"
)

type MetricsSvcImpl struct {
	sync.Mutex
	metricssvc.UnimplementedMetricsServiceServer
	clients []HealthInterface
}

func (m *MetricsSvcImpl) GetGPUState(ctx context.Context, req *metricssvc.GPUGetRequest) (*metricssvc.GPUStateResponse, error) {
	m.Lock()
	defer m.Unlock()
	//logger.Log.Printf("Got GetReq : %+v", req)
	resp := &metricssvc.GPUStateResponse{
		GPUState: []*metricssvc.GPUState{},
	}
	for _, client := range m.clients {
		gpuState, err := client.GetGPUHealthStates()
		if err != nil {
			return nil, err
		}
		for _, id := range req.ID {
			if gstate, ok := gpuState[id]; ok {
				state := &metricssvc.GPUState{
					ID:     id,
					Health: gstate,
				}
				resp.GPUState = append(resp.GPUState, state)
			}
		}
	}
	return resp, nil
}

func (m *MetricsSvcImpl) List(ctx context.Context, e *emptypb.Empty) (*metricssvc.GPUStateResponse, error) {
	m.Lock()
	defer m.Unlock()
	//logger.Log.Printf("Got ListReq")
	resp := &metricssvc.GPUStateResponse{
		GPUState: []*metricssvc.GPUState{},
	}

	for _, client := range m.clients {
		gpuState, err := client.GetGPUHealthStates()
		if err != nil {
			return nil, err
		}
		for gpu, state := range gpuState {
			gstate := &metricssvc.GPUState{
				ID:     gpu,
				Health: state,
			}
			resp.GPUState = append(resp.GPUState, gstate)
		}
	}
	return resp, nil
}

func (m *MetricsSvcImpl) SetGPUHealth(ctx context.Context, req *metricssvc.GPUUpdateRequest) (*metricssvc.GPUUpdateRequest, error) {
	m.Lock()
	defer m.Unlock()
	logger.Log.Printf("Got SetReq : %+v", req)
	if len(req.ID) != len(req.Health) {
		return nil, fmt.Errorf("invalid config mismatching id and state encountered")
	}
	for i, gpu := range req.ID {
		for _, client := range m.clients {
			_ = client.SetMockGPUHealthState(gpu, strings.ToLower(req.Health[i]))
		}
	}

	return req, nil

}

func (m *MetricsSvcImpl) mustEmbedUnimplementedMetricsServiceServer() {}

func newMetricsServer() *MetricsSvcImpl {
	msrv := &MetricsSvcImpl{
		clients: []HealthInterface{},
	}
	return msrv
}

func (m *MetricsSvcImpl) RegisterHealthClient(client HealthInterface) error {
	m.clients = append(m.clients, client)
	return nil
}
