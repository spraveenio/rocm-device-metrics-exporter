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
	"fmt"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/gen/amdgpu"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/gen/metricssvc"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/logger"
)

func (ga *GPUAgentClient) processHealthValidation() error {
	var gpumetrics *amdgpu.GPUGetResponse
	var evtData *amdgpu.EventResponse
	var err error

	errOccured := false
	ga.Lock()
	ga.healthState = make(map[string]string)
	ga.Unlock()

	gpuUUIDMap := make(map[string]string)
	gpuHealthState := make(map[string]bool)
	metricErrCheck := func(gpuid string, count float64) {
		if count > 0 {
			// set health to unhealthy
			gpuHealthState[gpuid] = false
			logger.Log.Printf("gpuid[%v] is set to unhealthy for ecc error", gpuid)
		}
	}

	eventErrCheck := func(e *amdgpu.Event) {
		uuid, _ := uuid.FromBytes(e.GPU)
		gpuuid := uuid.String()
		ts := e.Time.AsTime().Format(time.RFC3339)
		logger.Log.Printf("evt id=%v gpuid=%v severity=%v TimeStamp=%v Description=%v",
			e.Id, gpuuid, e.Severity, ts, e.Description)
		if e.Severity == amdgpu.EventSeverity_EVENT_SEVERITY_CRITICAL {
			if gpuid, ok := gpuUUIDMap[gpuuid]; ok {
				gpuHealthState[gpuid] = false
				logger.Log.Printf("gpuid[%v] is set to unhealthy for evt[%+v]", gpuid, e)
			} else {
				logger.Log.Printf("ignoring invalid gpuid[%v] is set to unhealthy for evt[%+v]", gpuuid, e)
			}
		}
	}

	gpumetrics, err = ga.getMetrics()
	if err != nil || (gpumetrics != nil && gpumetrics.ApiStatus != 0) {
		errOccured = true
		logger.Log.Printf("gpuagent get metrics failed %v", err)
		goto ret
	} else {
		// reset for every gpu state
		for _, gpu := range gpumetrics.Response {
			uuid, _ := uuid.FromBytes(gpu.Spec.Id)
			gpuid := fmt.Sprintf("%v", gpu.Status.Index)
			gpuuid := uuid.String()
			gpuUUIDMap[gpuuid] = gpuid
			gpuHealthState[gpuid] = true
			stats := gpu.Stats

			// business logic for health detection
			metricErrCheck(gpuid, normalizeUint64(stats.TotalCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.TotalUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.SDMACorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.SDMAUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.GFXCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.GFXUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.MMHUBCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.MMHUBUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.ATHUBCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.ATHUBUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.BIFCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.BIFUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.HDPCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.HDPUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.XGMIWAFLCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.XGMIWAFLUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.DFCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.DFUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.SMNCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.SMNUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.SEMCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.SEMUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.MP0CorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.MP0UncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.MP1CorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.MP1UncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.FUSECorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.FUSEUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.UMCCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.UMCUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.MCACorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.MCAUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.VCNCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.VCNUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.JPEGCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.JPEGUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.IHCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.IHUncorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.MPIOCorrectableErrors))
			metricErrCheck(gpuid, normalizeUint64(stats.MPIOUncorrectableErrors))
		}
	}

	evtData, err = ga.getEvents(amdgpu.EventSeverity_EVENT_SEVERITY_CRITICAL)
	if err != nil || (evtData != nil && evtData.ApiStatus != 0) {
		errOccured = true
		logger.Log.Printf("gpuagent get events failed %v", err)
	} else {
		// business logic for health detection
		for _, evt := range evtData.Event {
			eventErrCheck(evt)
		}
	}

ret:
	// disconnect on error
	if errOccured {
		ga.Close()
		// set state to unknown
		return fmt.Errorf("data pull error occured")
	}

	ga.Lock()
	for gpuid, healthy := range gpuHealthState {
		// override mock stats
		if mhealth, ok := ga.mockHealthState[gpuid]; ok {
			ga.healthState[gpuid] = mhealth
			continue
		}
		if healthy {
			ga.healthState[gpuid] = "healthy"
		} else {
			ga.healthState[gpuid] = "unhealthy"
		}
	}
	ga.Unlock()

	// logger.Log.Printf("health process update done :%+v", ga.healthState)

	return nil
}

func (ga *GPUAgentClient) SetMockGPUHealthState(gpuid, state string) error {
	ga.Lock()
	defer ga.Unlock()
	if _, ok := metricssvc.GPUHealth_value[strings.ToUpper(state)]; !ok {
		delete(ga.mockHealthState, gpuid)
	} else {
		ga.mockHealthState[gpuid] = state
	}
	return nil
}

func (ga *GPUAgentClient) GetGPUHealthStates() (map[string]string, error) {
	ga.Lock()
	defer ga.Unlock()
	if len(ga.healthState) == 0 {
		return nil, fmt.Errorf("health status not available")
	}
	healthMap := make(map[string]string)
	for id, state := range ga.healthState {
		healthMap[id] = state
	}

	return healthMap, nil
}
