
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

package gpuagent

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/pensando/device-metrics-exporter/internal/k8s"

	"github.com/gofrs/uuid"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/gen/amdgpu"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/logger"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/metricsutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	maxWorkers  = 5
	maxJobQueue = 16
)

type GPUAgentClient struct {
	conn         *grpc.ClientConn
	mh           *metricsutil.MetricsHandler
	client       amdgpu.GPUSvcClient
	m            *metrics // client specific metrics
	kubeClient   k8s.PodResourcesService
	isKubernetes bool
	sync.Mutex
	cacheGpuids map[string][]byte
	jobReqChan  chan []byte
	resultChan  chan *amdgpu.GPUGetResponse
}

func NewAgent(mh *metricsutil.MetricsHandler) (*GPUAgentClient, error) {
	agentAddr := mh.GetAgentAddr()
	logger.Log.Printf("Agent connecting to %v", agentAddr)
	conn, err := grpc.NewClient(agentAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Log.Printf("err :%v", err)
		return nil, err
	}
	client := amdgpu.NewGPUSvcClient(conn)

	ga := &GPUAgentClient{
		conn:   conn,
		client: client,
		mh:     mh,
	}

	if k8s.IsKubernetes() {
		kubeClient, err := k8s.NewClient()
		if err != nil {
			return nil, fmt.Errorf("error in kubelet client, %v", err)
		}
		ga.isKubernetes = true
		ga.kubeClient = kubeClient
	}

	ga.cacheGpuids = make(map[string][]byte)
	ga.jobReqChan = make(chan []byte, maxJobQueue)
	ga.resultChan = make(chan *amdgpu.GPUGetResponse, maxJobQueue)
	mh.RegisterMetricsClient(ga)

	totalWorkers := maxWorkers
	numCores := runtime.NumCPU()

	if numCores < maxWorkers {
		totalWorkers = numCores
	}
	logger.Log.Printf("total workers[%v] queue size[%v]", totalWorkers, maxJobQueue)
	// create 3 workers
	for i := 1; i <= maxWorkers; i++ {
		go ga.workerInit(i)
	}
	return ga, nil
}

func (ga *GPUAgentClient) workerInit(id int) {
	for gpuReq := range ga.jobReqChan {
		uuid, _ := uuid.FromBytes(gpuReq)
		req := &amdgpu.GPUGetRequest{
			Id: [][]byte{
				gpuReq,
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*5))
		res, err := ga.client.GPUGet(ctx, req)
		cancel()
		if err != nil {
			res = nil
			logger.Log.Printf("worker[%d] job[%d] err %v", id, uuid, err)
		}
		// result can be nil
		ga.resultChan <- res
	}
}

func (ga *GPUAgentClient) getMetricsBulkReq() error {
	// create multiple workers
	// keep pushing jobs based upon the available workers
	numReq := len(ga.cacheGpuids)
	for _, gpuid := range ga.cacheGpuids {
		ga.jobReqChan <- gpuid
	}

	for i := 1; i <= numReq; i++ {
		gpuRes := <-ga.resultChan
		if gpuRes != nil && len(gpuRes.Response) > 0 {
			ga.updateGPUInfoToMetrics(gpuRes.Response[0])
		}
	}
	return nil
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
	close(ga.jobReqChan)
	close(ga.resultChan)
}
