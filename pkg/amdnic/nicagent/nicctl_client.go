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
	"sync"
)

type NICCtlClient struct {
	sync.Mutex
	na *NICAgentClient
}

var tempVar int // To be removed

func newNICCtlClient(na *NICAgentClient) *NICCtlClient {
	nc := &NICCtlClient{na: na}
	return nc
}

func (nc *NICCtlClient) Init() error {
	nc.Lock()
	defer nc.Unlock()
	// TODO check nicctl connection to NIC cards and return error for failure
	return nil
}

func (nc *NICCtlClient) UpdateNICStats() error {
	var wg sync.WaitGroup
	fn_ptrs := []func() error{nc.UpdateCardStats, nc.UpdatePortStats, nc.UpdateLifStats}

	for _, fn := range fn_ptrs {
		wg.Add(1)
		go func(f func() error) {
			defer wg.Done()
			f()
		}(fn)
	}
	wg.Wait()
	return nil
}

func (nc *NICCtlClient) UpdateCardStats() error {
	nc.Lock()
	defer nc.Unlock()
	tempVar += 1
	nc.na.m.nicNodesTotal.Set(float64(tempVar))
	return nil
}

func (nc *NICCtlClient) UpdatePortStats() error {
	nc.Lock()
	defer nc.Unlock()
	labels := nc.na.populateLabelsFromNIC()
	nc.na.m.nicMaxSpeed.With(labels).Set(float64(400))
	return nil
}

func (nc *NICCtlClient) UpdateLifStats() error {
	return nil
}
