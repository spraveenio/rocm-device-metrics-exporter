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
	"encoding/json"
	"os/exec"
	"sync"

	"github.com/ROCm/device-metrics-exporter/pkg/amdnic/gen/nicmetrics"
	"github.com/ROCm/device-metrics-exporter/pkg/amdnic/nicagent/utils"
	"github.com/ROCm/device-metrics-exporter/pkg/exporter/logger"
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

	portStatsOut, err := exec.Command("/bin/bash", "-c", "nicctl show port statistics -j").Output()
	if err != nil {
		logger.Log.Printf("failed to get port statistics, err: %+v", err)
		return err
	}

	// Unmarshal the JSON data into the port statistics
	var portStats nicmetrics.PortStatsList
	err = json.Unmarshal(portStatsOut, &portStats)
	if err != nil {
		logger.Log.Printf("error unmarshaling port statistics data: %v", err)
		return err
	}

	// for each reported port stats, find out the port name and report metrics to prometheus
	for _, nic := range portStats.NIC {
		labels := nc.na.populateLabelsFromNIC(nic.ID)
		for _, port := range nic.Port {
			portName := nc.na.nics[nic.ID].Ports[port.ID].Name
			portID := nc.na.nics[nic.ID].Ports[port.ID].Index
			labels[LabelPortName] = portName
			labels[LabelPortID] = portID

			// rx counters
			nc.na.m.nicPortStatsFramesRxBadFcs.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_RX_BAD_FCS)))
			nc.na.m.nicPortStatsFramesRxBadAll.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_RX_BAD_ALL)))
			nc.na.m.nicPortStatsFramesRxPause.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_RX_PAUSE)))
			nc.na.m.nicPortStatsFramesRxBadLength.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_RX_BAD_LENGTH)))
			nc.na.m.nicPortStatsFramesRxUndersized.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_RX_UNDERSIZED)))
			nc.na.m.nicPortStatsFramesRxOversized.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_RX_OVERSIZED)))
			nc.na.m.nicPortStatsFramesRxFragments.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_RX_FRAGMENTS)))
			nc.na.m.nicPortStatsFramesRxJabber.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_RX_JABBER)))
			nc.na.m.nicPortStatsFramesRxPripause.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_RX_PRIPAUSE)))
			nc.na.m.nicPortStatsFramesRxStompedCrc.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_RX_STOMPED_CRC)))
			nc.na.m.nicPortStatsFramesRxTooLong.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_RX_TOO_LONG)))
			nc.na.m.nicPortStatsFramesRxDropped.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_RX_DROPPED)))

			//tx counter
			nc.na.m.nicPortStatsFramesTxBad.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_TX_BAD)))
			nc.na.m.nicPortStatsFramesTxPause.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_TX_PAUSE)))
			nc.na.m.nicPortStatsFramesTxPripause.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_TX_PRIPAUSE)))
			nc.na.m.nicPortStatsFramesTxLessThan64b.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_TX_LESS_THAN_64B)))
			nc.na.m.nicPortStatsFramesTxTruncated.With(labels).Set(float64(utils.StringToUint64(port.FRAMES_TX_TRUNCATED)))
			nc.na.m.nicPortStatsRsfecCorrectableWord.With(labels).Set(float64(utils.StringToUint64(port.RSFEC_CORRECTABLE_WORD)))
			nc.na.m.nicPortStatsRsfecChSymbolErrCnt.With(labels).Set(float64(utils.StringToUint64(port.RSFEC_CH_SYMBOL_ERR_CNT)))
		}
	}

	return nil
}

func (nc *NICCtlClient) UpdateLifStats() error {
	return nil
}
