/*
Copyright (c) Advanced Micro Devices, Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the \"License\");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an \"AS IS\" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"strconv"

	"github.com/ROCm/device-metrics-exporter/pkg/exporter/logger"
)

func StringToUint64(str string) uint64 {
	val, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		logger.Log.Printf("error converting string to uint64, err: %v", err)
		return 0
	}
	return val
}
