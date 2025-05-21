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

package fsysdevice

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"

	"github.com/ROCm/device-metrics-exporter/pkg/exporter/logger"
)

const (
	AMDLogicalDevicePrefix = "amdgpu_xcp_"
	AMDGPURenderStartID    = 128
)

var (
	once sync.Once
)

// FindAMDGPUDevices scans the system for AMDGPU XCP devices and returns a map
// where the key is "gpu_id" and value is device name "amdgpu_xcp_N"
func FindAMDGPUDevices() (map[string]string, error) {
	result := make(map[string]string)

	basePattern := "/sys/devices/platform/amdgpu_xcp_*/drm/renderD*"
	matches, err := filepath.Glob(basePattern)
	if err != nil {
		return nil, fmt.Errorf("glob error: %w", err)
	}

	// Regex to extract amdgpu_xcpN and renderDN
	xcpRe := regexp.MustCompile(`amdgpu_xcp_(\d+)`)
	renderRe := regexp.MustCompile(`renderD(\d+)`)

	for _, path := range matches {
		// Check if the path exists and is a directory or symlink
		if _, err := os.Stat(path); err != nil {
			continue
		}

		// Extract amdgpu_xcpN
		xcpMatch := xcpRe.FindStringSubmatch(path)
		renderMatch := renderRe.FindStringSubmatch(path)
		if len(xcpMatch) < 2 || len(renderMatch) < 2 {
			continue
		}

		xcpVal := AMDLogicalDevicePrefix + xcpMatch[1]
		renderKey, err := strconv.Atoi(renderMatch[1])
		if err != nil {
			continue
		}

		gpuID := fmt.Sprintf("%v", renderKey%AMDGPURenderStartID)
		result[gpuID] = xcpVal
	}

	return result, nil
}

type FsysDevice struct {
	lgpuMap map[string]string
}

var FsysDeviceHandler *FsysDevice

func GetFsysDeviceHandler() *FsysDevice {
	if FsysDeviceHandler == nil {
		FsysDeviceHandler = &FsysDevice{
			lgpuMap: make(map[string]string),
		}
		FsysDeviceHandler.init()
	}
	return FsysDeviceHandler
}

func (fs *FsysDevice) init() {
	lgpuMap, err := FindAMDGPUDevices()
	if err != nil {
		logger.Log.Printf("FindAMDGPUDevices error :%v", err)
		return
	}
	fs.lgpuMap = lgpuMap
}

func (fs *FsysDevice) GetDeviceNameFromID(gpuid string) (string, error) {
	if devicename, ok := fs.lgpuMap[gpuid]; ok {
		return devicename, nil
	}
	return "", fmt.Errorf("device not found")
}
