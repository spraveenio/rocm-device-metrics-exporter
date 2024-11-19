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
	"log"
	"net"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/gen/metricssvc"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/globals"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/logger"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type metricsSvcImpl struct {
	sync.Mutex
	gpuState map[string]string
	metricssvc.UnimplementedMetricsServiceServer
}

func (m *metricsSvcImpl) GetGPUState(ctx context.Context, req *metricssvc.GPUStateRequest) (*metricssvc.GPUStateResponse, error) {
	m.Lock()
	defer m.Unlock()
	logger.Log.Printf("Got GetReq : %+v", req)
	resp := &metricssvc.GPUStateResponse{
		GPUState: []*metricssvc.GPUState{},
	}
	// mock it for now
	for _, gpu := range req.ID {
		gstate := &metricssvc.GPUState{
			ID:     gpu,
			Health: m.gpuState[gpu],
		}
		resp.GPUState = append(resp.GPUState, gstate)
	}
	return resp, nil
}

func (m *metricsSvcImpl) List(ctx context.Context, e *emptypb.Empty) (*metricssvc.GPUStateResponse, error) {
	m.Lock()
	defer m.Unlock()
	logger.Log.Printf("Got ListReq")
	resp := &metricssvc.GPUStateResponse{
		GPUState: []*metricssvc.GPUState{},
	}
	// mock it for now
	for gpu, state := range m.gpuState {
		gstate := &metricssvc.GPUState{
			ID:     gpu,
			Health: state,
		}
		resp.GPUState = append(resp.GPUState, gstate)
	}
	return resp, nil
}
func (m *metricsSvcImpl) SetGPUHealth(ctx context.Context, req *metricssvc.GPUUpdateRequest) (*metricssvc.GPUStateResponse, error) {
	m.Lock()
	defer m.Unlock()
	logger.Log.Printf("Got SetReq : %+v", req)
	if len(req.ID) != len(req.Health) {
		return nil, fmt.Errorf("invalid config mismatching id and state encountered")
	}
	for i, gpu := range req.ID {
		m.gpuState[gpu] = strings.ToLower(req.Health[i])
	}

	resp := &metricssvc.GPUStateResponse{
		GPUState: []*metricssvc.GPUState{},
	}
	// mock it for now
	for _, gpu := range req.ID {
		gstate := &metricssvc.GPUState{
			ID:     gpu,
			Health: m.gpuState[gpu],
		}
		resp.GPUState = append(resp.GPUState, gstate)
	}
	return resp, nil

}

func (m *metricsSvcImpl) mustEmbedUnimplementedMetricsServiceServer() {}

func NewMetricsServer() *grpc.Server {
	gsrv := grpc.NewServer()
	srv := &metricsSvcImpl{}
	srv.gpuState = make(map[string]string)
	for i := 1; i <= 16; i = i + 1 {
		srv.gpuState[fmt.Sprintf("%d", i)] = strings.ToLower(metricssvc.GPUHealth_HEALTHY.String())
	}
	metricssvc.RegisterMetricsServiceServer(gsrv, srv)
	return gsrv
}

func Run() {
	socketPath := globals.MetricsSocketPath
	// Remove any existing socket file
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("Failed to remove socket file: %v", err)
	}

	os.MkdirAll(path.Dir(socketPath), 0755)

	logger.Log.Printf("starting listening on socket : %v", socketPath)
	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("failed to listen on port: %v", err)
	}
	logger.Log.Printf("Listening on socket %v", socketPath)
	srv := NewMetricsServer()

	if err := srv.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
