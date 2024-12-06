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
	"os"
	"sync"
	"time"

	"github.com/pensando/device-metrics-exporter/pkg/slurm"

	"github.com/pensando/device-metrics-exporter/pkg/k8s"

	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/gen/amdgpu"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/k8sclient"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/logger"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/metricsutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// cachgpuid are updated after this many pull request
	refreshInterval = 30 * time.Second
)

type GPUAgentClient struct {
	sync.Mutex
	conn             *grpc.ClientConn
	mh               *metricsutil.MetricsHandler
	gpuclient        amdgpu.GPUSvcClient
	evtclient        amdgpu.EventSvcClient
	m                *metrics // client specific metrics
	kubeClient       k8s.PodResourcesService
	k8sLabelClient   *k8sclient.K8sClient
	isKubernetes     bool
	slurmClient      slurm.JobsService
	staticHostLabels map[string]string
	ctx              context.Context
	cancel           context.CancelFunc
	healthState      map[string]string
	mockEccField     map[string]map[string]uint32 // gpuid->fields->count
}

func initclients(mh *metricsutil.MetricsHandler) (conn *grpc.ClientConn, gpuclient amdgpu.GPUSvcClient, evtclient amdgpu.EventSvcClient, err error) {
	agentAddr := mh.GetAgentAddr()
	logger.Log.Printf("Agent connecting to %v", agentAddr)
	conn, err = grpc.NewClient(agentAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Log.Printf("err :%v", err)
		return
	}
	gpuclient = amdgpu.NewGPUSvcClient(conn)
	evtclient = amdgpu.NewEventSvcClient(conn)
	return
}

func initSchedulers(ctx context.Context) (kubeClient k8s.PodResourcesService, slurmClient slurm.JobsService, err error) {
	if k8s.IsKubernetes() {
		kubeClient, err = k8s.NewClient()
		if err != nil {
			return nil, nil, fmt.Errorf("error in kubelet client, %v", err)
		}
	} else {
		slurmClient, err = slurm.NewClient(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("error in slurm client, %v", err)
		}
	}
	return
}

func NewAgent(mh *metricsutil.MetricsHandler) *GPUAgentClient {
	ga := &GPUAgentClient{mh: mh}
	ga.healthState = make(map[string]string)
	ga.mockEccField = make(map[string]map[string]uint32)
	mh.RegisterMetricsClient(ga)
	return ga
}

func (ga *GPUAgentClient) init() error {
	ga.Lock()
	defer ga.Unlock()
	ga.initializeContext()
	ga.healthState = make(map[string]string)
	conn, gpuclient, evtclient, err := initclients(ga.mh)
	if err != nil {
		logger.Log.Printf("gpu client init failure err :%v", err)
		return err
	}

	ga.conn = conn
	ga.gpuclient = gpuclient
	ga.evtclient = evtclient

	k8c, sc, err := initSchedulers(ga.ctx)
	if err != nil {
		logger.Log.Printf("gpu client init failure err :%v", err)
		return err
	}

	if k8c != nil {
		ga.isKubernetes = true
		ga.kubeClient = k8c
		ga.k8sLabelClient = k8sclient.NewClient()
	} else {
		ga.isKubernetes = false
		ga.slurmClient = sc
	}

	if err := ga.populateStaticHostLabels(); err != nil {
		return fmt.Errorf("error in populating static host labels, %v", err)
	}

	logger.Log.Printf("monitor %v jobs", map[bool]string{true: "kubernetes", false: "slurm"}[ga.isKubernetes])

	return nil
}

func (ga *GPUAgentClient) initializeContext() {
	ctx, cancel := context.WithCancel(context.Background())
	ga.ctx = ctx
	ga.cancel = cancel
}

func (ga *GPUAgentClient) reconnect() error {
	ga.Close()
	return ga.init()
}

func (ga *GPUAgentClient) isActive() bool {
	ga.Lock()
	defer ga.Unlock()
	return ga.gpuclient != nil
}

func (ga *GPUAgentClient) StartMonitor() {
	logger.Log.Printf("GPUAgent monitor started")
	ga.initializeContext()
	pollTimer := time.NewTicker(refreshInterval)
	defer pollTimer.Stop()

	for {
		select {
		case <-ga.ctx.Done():
			logger.Log.Printf("gpuagent client connection closing")
			ga.Close()
			return
		case <-pollTimer.C:
			if !ga.isActive() {
				if err := ga.reconnect(); err != nil {
					logger.Log.Printf("gpuagent connection failed %v", err)
					continue
				}
			}
			ga.processHealthValidation()
			ga.sendNodeLabelUpdate()
		}
	}
}

func (ga *GPUAgentClient) sendNodeLabelUpdate() error {
	if !ga.isKubernetes {
		return nil
	}
	// send update to label , reconnect logic tbd
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		logger.Log.Printf("error getting node name on k8s deployment, skip label update")
		return fmt.Errorf("node name not found")
	}
	gpuHealthStates := make(map[string]string)
	ga.Lock()
	for gpuid, state := range ga.healthState {
		gpuHealthStates[gpuid] = state
	}
	ga.Unlock()
	_ = ga.k8sLabelClient.UpdateHealthLabel(nodeName, gpuHealthStates)
	return nil
}

func (ga *GPUAgentClient) getMetricsAll() error {
	// send the req to gpuclient
	resp, err := ga.getMetrics()
	if err != nil {
		return err
	}
	if resp != nil && resp.ApiStatus != 0 {
		logger.Log.Printf("resp status :%v", resp.ApiStatus)
		return fmt.Errorf("%v", resp.ApiStatus)
	}
	for _, gpu := range resp.Response {
		ga.updateGPUInfoToMetrics(gpu)
	}

	return nil
}

func (ga *GPUAgentClient) getMetrics() (*amdgpu.GPUGetResponse, error) {
	if !ga.isActive() {
		ga.reconnect()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &amdgpu.GPUGetRequest{}
	res, err := ga.gpuclient.GPUGet(ctx, req)
	return res, err
}

func (ga *GPUAgentClient) getEvents(severity amdgpu.EventSeverity) (*amdgpu.EventResponse, error) {
	req := &amdgpu.EventRequest{}
	if severity != amdgpu.EventSeverity_EVENT_SEVERITY_NONE {
		req.Filter = &amdgpu.EventFilter{
			Filter: &amdgpu.EventFilter_MatchAttrs{
				MatchAttrs: &amdgpu.EventMatchAttrs{
					Severity: severity,
				},
			},
		}
	}
	res, err := ga.evtclient.EventGet(ga.ctx, req)
	return res, err
}

func (ga *GPUAgentClient) Close() {
	ga.Lock()
	defer ga.Unlock()
	if ga.conn != nil {
		ga.conn.Close()
		ga.gpuclient = nil
		ga.conn = nil
	}
	if ga.kubeClient != nil {
		ga.kubeClient.Close()
		ga.kubeClient = nil
	}

	if ga.slurmClient != nil {
		ga.slurmClient.Close()
		ga.slurmClient = nil
	}
}
