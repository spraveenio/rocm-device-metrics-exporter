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
	"context"
	"encoding/json"
	"flag"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pensando/device-metrics-exporter/internal/testrunner"
	types "github.com/pensando/device-metrics-exporter/internal/testrunner/interface"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/gen/metricssvc"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/globals"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/logger"
)

const (
	logPrefix          = "test-runner "
	logFile            = "test-runner.log"
	logDir             = "/var/run/"
	gpuStateWatchFreq  = 5 * time.Second
	gpuStateReqTimeout = 10 * time.Second
)

var (
	globalTestConfig = types.TestParams{
		Iterations:    1,
		StopOnFailure: false,
		DeviceIDs:     []string{},
		ExtraArgs:     []string{},
	}
	rvsTestRunner types.TestRunner
)

func main() {
	var (
		rvsPath            = flag.String("rvs-path", globals.RVSPath, "Path to ROCmValidationSuite rvs binary file")
		testCaseDir        = flag.String("test-case-dir", globals.AMDTestCaseDir, "AMD GPU test case directory")
		testConfigPath     = flag.String("test-cfg-path", globals.AMDTestCfgPath, "Path to test runner config path")
		exporterSocketPath = flag.String("exporter-socket-path", globals.MetricsSocketPath, "Path to exporter metrics server docket")
	)

	flag.Parse()
	logger.SetLogDir(logDir)
	logger.SetLogFile(logFile)
	logger.SetLogPrefix(logPrefix)
	logger.Init()

	// parse test config
	// users could specify the test config json at /etc/rvs/config.json
	// if not specified the test runner will use default config
	globalTestConfig = parseTestCfg(*testConfigPath)

	// init test runner
	runner, err := testrunner.NewRvsTestRunner(*rvsPath, *testCaseDir, logDir)
	if err != nil || runner == nil {
		logger.Log.Printf("failed to create rvs test runner, runner: %+v, err: %+v", runner, err)
		return
	}
	rvsTestRunner = runner

	// listen for unix socket to receive the event for triggering the test run
	watchGPUState(*exporterSocketPath)
}

func parseTestCfg(path string) types.TestParams {
	var testCfg types.TestParams
	if _, err := os.Stat(path); err != nil {
		logger.Log.Printf("failed to get test runner config at %+v err: %+v, start to use default testConfig %+v", path, err, globalTestConfig)
		return globalTestConfig
	} else if testCfgbytes, err := os.ReadFile(path); err != nil {
		logger.Log.Printf("failed to read test runner config at %+v err: %+v, start to use default testConfig %+v", path, err, globalTestConfig)
		return globalTestConfig
	} else if err = json.Unmarshal(testCfgbytes, &testCfg); err != nil {
		logger.Log.Printf("failed to parse test runner config at %+v err: %+v, start to use default testConfig %+v", path, err, globalTestConfig)
		return globalTestConfig
	}
	return testCfg
}

func watchGPUState(socketPath string) {
	conn, err := grpc.NewClient("unix:"+socketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Log.Fatalf("testrunner cannot connect to %v: %v", "unix:"+socketPath, err)
		return
	}
	defer conn.Close()
	c := metricssvc.NewMetricsServiceClient(conn)

	timer := time.NewTicker(gpuStateWatchFreq)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			ctx, cancel := context.WithTimeout(context.Background(), gpuStateReqTimeout)
			r, err := c.List(ctx, &emptypb.Empty{})
			if err != nil {
				logger.Log.Fatalf("could not list GPU state: %v", err)
				cancel()
				return
			}
			logger.Log.Printf("GPU State: %s", r.String())
			cancel()

			unHealthyGPUIDs := []string{}
			if r != nil {
				for _, state := range r.GPUState {
					// if any GPU is not healthy, start a test against those GPUs
					if state.Health != metricssvc.GPUHealth_HEALTHY.String() {
						unHealthyGPUIDs = append(unHealthyGPUIDs, state.ID)
					}
				}
			}

			// start test on unhealthy GPU
			if len(globalTestConfig.DeviceIDs) > 0 {
				logger.Log.Printf("test config force to run test for GPU %+v", globalTestConfig.DeviceIDs)
				go testGPU(globalTestConfig.DeviceIDs)
			} else if len(unHealthyGPUIDs) > 0 {
				logger.Log.Printf("found GPU with unhealthy state %+v", unHealthyGPUIDs)
				go testGPU(unHealthyGPUIDs)
			}
		}
	}
}

func testGPU(ids []string) {
	handler, err := rvsTestRunner.GetTestHandler("rvs", types.TestParams{
		Iterations:    globalTestConfig.Iterations,
		StopOnFailure: globalTestConfig.StopOnFailure,
		DeviceIDs:     ids,
		Timeout:       globalTestConfig.Timeout,
		ExtraArgs:     globalTestConfig.ExtraArgs,
	})
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
		// for now updating same result in all GPU
		result := handler.Result()
		logger.Log.Printf("TestRun: Name: %v suite %v completed. Result: %v", "test123", "rvs123", result)
	}
}
