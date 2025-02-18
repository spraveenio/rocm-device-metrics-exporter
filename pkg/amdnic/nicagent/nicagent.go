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

package nicagent

import (
	"context"
	"sync"

	"github.com/ROCm/device-metrics-exporter/pkg/exporter/logger"
	"github.com/ROCm/device-metrics-exporter/pkg/exporter/metricsutil"
	"github.com/ROCm/device-metrics-exporter/pkg/exporter/utils"
)

type NICAgentClient struct {
	sync.Mutex
	nicClients   []NICInterface
	mh           *metricsutil.MetricsHandler
	m            *metrics // client specific metrics
	isKubernetes bool
	ctx          context.Context
	cancel       context.CancelFunc
}

func (na *NICAgentClient) initClients() (err error) {
	logger.Log.Printf("Establishing connection to NIC clients")
	for _, client := range na.nicClients {
		if err = client.Init(); err != nil {
			logger.Log.Printf("NICCTL client init err :%+v", err)
			return err
		}
	}
	return
}

func NewAgent(mh *metricsutil.MetricsHandler) *NICAgentClient {
	na := &NICAgentClient{mh: mh}
	na.nicClients = []NICInterface{}
	return na
}

func (na *NICAgentClient) Init() error {
	na.Lock()
	defer na.Unlock()
	na.initializeContext()

	nicCtlClient := newNICCtlClient(na)
	na.nicClients = append(na.nicClients, nicCtlClient)

	err := na.initClients()
	if err != nil {
		logger.Log.Printf("NIC client init failure err :%v", err)
		return err
	}

	na.mh.RegisterMetricsClient(na)
	if utils.IsKubernetes() {
		na.isKubernetes = true
	}

	return nil
}

func (na *NICAgentClient) initializeContext() {
	ctx, cancel := context.WithCancel(context.Background())
	na.ctx = ctx
	na.cancel = cancel
}

func (na *NICAgentClient) getMetricsAll() error {
	var wg sync.WaitGroup
	for _, client := range na.nicClients {
		wg.Add(1)
		go func(client NICInterface) {
			defer wg.Done()
			client.UpdateNICStats()
		}(client)
	}
	wg.Wait()
	return nil
}
