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

package main

import (
	"encoding/json"
	"flag"
	"os"

	"github.com/pensando/device-metrics-exporter/internal/testrunner"
	types "github.com/pensando/device-metrics-exporter/internal/testrunner/interface"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/globals"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/logger"
)

const (
	logPrefix = "test-runner "
	logFile   = "test-runner.log"
	logDir    = "/var/run/"
)

var defaultTestConfig = types.TestParams{
	Iterations:    1,
	StopOnFailure: false,
	DeviceIDs:     []string{},
	ExtraArgs:     []string{},
}

func main() {
	var (
		rvsPath        = flag.String("rvs-path", globals.RVSPath, "Path to ROCmValidationSuite rvs binary file")
		testCaseDir    = flag.String("test-case-dir", globals.AMDTestCaseDir, "AMD GPU test case directory")
		testConfigPath = flag.String("test-cfg-path", globals.AMDTestCfgPath, "Path to test runner config path")
		//exporterGrpcPort = flag.Int("exporter-grpc-port", globals.ExporterGRPCPort, "Exporter GRPC port")
	)
	flag.Parse()
	logger.SetLogDir(logDir)
	logger.SetLogFile(logFile)
	logger.SetLogPrefix(logPrefix)
	logger.Init()

	// listen for unix socket to receive the event for triggering the test run

	// parse test config
	testCfg := parseTestCfg(*testConfigPath)

	// init test runner
	runner, err := testrunner.NewRvsTestRunner(*rvsPath, *testCaseDir, logDir)
	if err != nil {
		logger.Log.Printf("failed to create test runner, err: %+v", err)
		return
	}

	handler, err := runner.GetTestHandler("rvs", testCfg)
	if err != nil {
		logger.Log.Printf("failed to get test run handler, err: %+v", err)
		return
	}
	err = handler.StartTest()
	if err != nil {
		logger.Log.Printf("failed to start test run, err: %+v", err)
		return
	}

	select {
	case <-handler.Done():
		// TODO: this has to change later based on result logs parsing.
		// for now updating same result in all GPUs
		result := handler.Result()
		logger.Log.Printf("TestRun: Name: %v suite %v completed. Result: %v", "test123", "rvs123", result)
	}
}

func parseTestCfg(path string) types.TestParams {
	var testCfg types.TestParams
	if _, err := os.Stat(path); err != nil {
		logger.Log.Printf("failed to get test runner config at %+v err: %+v, start to use default testConfig %+v", path, err, defaultTestConfig)
		return defaultTestConfig
	} else if testCfgbytes, err := os.ReadFile(path); err != nil {
		logger.Log.Printf("failed to read test runner config at %+v err: %+v, start to use default testConfig %+v", path, err, defaultTestConfig)
		return defaultTestConfig
	} else if err = json.Unmarshal(testCfgbytes, &testCfg); err != nil {
		logger.Log.Printf("failed to parse test runner config at %+v err: %+v, start to use default testConfig %+v", path, err, defaultTestConfig)
		return defaultTestConfig
	}
	return testCfg
}
