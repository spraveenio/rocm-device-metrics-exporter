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
	"encoding/json"
	"fmt"
	"log"
	"os/exec"

	"github.com/ROCm/device-metrics-exporter/pkg/exporter/logger"
)

// GetPortName return port name given the port ID
func GetPortName(portID string) string {
	cmd := fmt.Sprintf("nicctl show port --port %s --json", portID)
	portData, err := exec.Command("/bin/bash", "-c", cmd).Output()
	if err != nil {
		logger.Log.Printf("failed to get port data, err: %+v", err)
		return ""
	}

	// hold the unmarshalled data as a map
	var data map[string]interface{}

	// Unmarshal the JSON data into the map
	err = json.Unmarshal(portData, &data)
	if err != nil {
		log.Fatalf("error unmarshaling port data: %v", err)
	}

	// dynamically extract the name field using the map
	if nic, ok := data["nic"].([]interface{}); ok {
		for _, n := range nic {
			if nicObj, ok := n.(map[string]interface{}); ok {
				if port, ok := nicObj["port"].([]interface{}); ok {
					for _, p := range port {
						if portObj, ok := p.(map[string]interface{}); ok {
							if spec, ok := portObj["spec"].(map[string]interface{}); ok {
								if name, ok := spec["name"].(string); ok {
									return name
								}
							}
						}
					}
				}
			}
		}
	}

	return ""
}
