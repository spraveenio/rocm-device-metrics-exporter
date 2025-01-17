/*
*
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
*
*/

package testrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/gen/metricssvc"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/globals"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/logger"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/utils"
	k8sclient "github.com/pensando/device-metrics-exporter/pkg/client"
	testrunnerGen "github.com/pensando/device-metrics-exporter/pkg/testrunner/gen/testrunner"
	types "github.com/pensando/device-metrics-exporter/pkg/testrunner/interface"
)

var (
	defaultGlobalTestRunnerConfig = &testrunnerGen.TestRunnerConfig{
		TestConfig: map[string]*testrunnerGen.TestCategoryConfig{
			testrunnerGen.TestCategory_GPU_HEALTH_CHECK.String(): {
				TestLocationTrigger: map[string]*testrunnerGen.TestTriggerConfig{
					globals.GlobalTestTriggerKeyword: {
						TestParameters: map[string]*testrunnerGen.TestParameters{
							testrunnerGen.TestTrigger_AUTO_UNHEALTHY_GPU_WATCH.String(): {
								TestCases: []*testrunnerGen.TestParameter{
									{
										Recipe:         globals.DefaultUnhealthyGPUTestName,
										Iterations:     globals.DefaultUnhealthyGPUTestIterations,
										StopOnFailure:  globals.DefaultUnhealthyGPUTestStopOnFailure,
										TimeoutSeconds: globals.DefaultUnhealthyGPUTestTimeoutSeconds,
									},
								},
							},
							testrunnerGen.TestTrigger_PRE_START_JOB_CHECK.String(): {
								TestCases: []*testrunnerGen.TestParameter{
									{
										Recipe:         globals.DefaultPreJobCheckTestName,
										Iterations:     globals.DefaultPreJobCheckTestIterations,
										StopOnFailure:  globals.DefaultPreJobCheckTestStopOnFailure,
										TimeoutSeconds: globals.DefaultPreJobCheckTestTimeoutSeconds,
									},
								},
							},
							testrunnerGen.TestTrigger_MANUAL.String(): {
								TestCases: []*testrunnerGen.TestParameter{
									{
										Recipe:         globals.DefaultManualTestName,
										Iterations:     globals.DefaultManualTestIterations,
										StopOnFailure:  globals.DefaultManualTestStopOnFailure,
										TimeoutSeconds: globals.DefaultManualTestTimeoutSeconds,
									},
								},
							},
						},
					},
				},
			},
		},
	}
)

type TestRunner struct {
	hostName           string
	rvsPath            string
	rocmSMIPath        string
	exporterSocketPath string

	testCategory   string
	testLocation   string
	testTrigger    string
	rvsTestCaseDir string
	testCfgPath    string

	logDir       string
	statusDBPath string

	sync.Mutex             // mutex to protect globalTestRunnerConfig from file watcher
	globalTestRunnerConfig *testrunnerGen.TestRunnerConfig
	rvsTestRunner          types.TestRunner

	// k8s related fields
	isK8s           bool
	k8sClient       *k8sclient.K8sClient
	k8sPodName      string
	k8sPodNamespace string
}

// initTestRunner init the test runner and related configs
// return the test location, either global or specific host name
func NewTestRunner(rvsPath, rvsTestCaseDir, rocmSMIPath, exporterSocketPath, testRunnerConfigPath, testCategory, testTrigger, logDir string) *TestRunner {
	runner := &TestRunner{
		rvsPath:            rvsPath,
		rocmSMIPath:        rocmSMIPath,
		exporterSocketPath: exporterSocketPath,
		testCategory:       testCategory,
		testTrigger:        testTrigger,
		testCfgPath:        testRunnerConfigPath,
		rvsTestCaseDir:     rvsTestCaseDir,
		logDir:             logDir,
	}
	// init test runner config
	// testRunnerConfigPath file existence has been verified
	runner.initLogger()
	runner.readTestRunnerConfig(testRunnerConfigPath)
	runner.getHostName()
	runner.validateTestTrigger()
	runner.initTestRunnerConfig()
	if utils.IsKubernetes() {
		runner.isK8s = true
		runner.k8sClient = k8sclient.NewClient(context.Background())
	}
	logger.Log.Printf("Test runner isKubernetes: %+v config: %+v", runner.isK8s, runner.globalTestRunnerConfig)
	return runner
}

// validateTestTrigger validates the test category/location/trigger existence
// return test locaiton, either global or specific hostname
func (tr *TestRunner) validateTestTrigger() {
	tr.Lock()
	defer tr.Unlock()

	// 1. verify test category
	// given category config should exist
	if tr.globalTestRunnerConfig.TestConfig == nil {
		fmt.Printf("failed to find any test category config from %+v\n", tr.globalTestRunnerConfig)
		os.Exit(1)
	}
	if _, ok := tr.globalTestRunnerConfig.TestConfig[tr.testCategory]; !ok {
		fmt.Printf("failed to find category %+v from config %+v\n", tr.testCategory, tr.globalTestRunnerConfig)
		os.Exit(1)
	}

	// 2. verify test location
	// global config or given hostname's config should exist
	categoryConfig := tr.globalTestRunnerConfig.TestConfig[tr.testCategory]
	if categoryConfig == nil {
		fmt.Printf("got empty config for test category %+v", tr.testCategory)
		os.Exit(1)
	}

	if categoryConfig.TestLocationTrigger == nil {
		fmt.Printf("failed to find any global or host specific test config under category %+v: %+v\n", tr.testCategory, categoryConfig)
		os.Exit(1)
	}
	_, foundHostSpecifcTest := categoryConfig.TestLocationTrigger[tr.hostName]
	_, foundGlobalTest := categoryConfig.TestLocationTrigger[globals.GlobalTestTriggerKeyword]
	if !foundGlobalTest && !foundHostSpecifcTest {
		fmt.Printf("cannot find neither global test config nor host specific config under category %+v: %+v\n", tr.testCategory, categoryConfig)
		os.Exit(1)
	}

	// 3. validate test trigger's config
	// if host specifc config was found
	// validate host specific config's trigger
	if foundHostSpecifcTest {
		if categoryConfig.TestLocationTrigger[tr.hostName].TestParameters == nil {
			fmt.Printf("failed to get any test trigger under category %+v config: %+v\n", categoryConfig, categoryConfig.TestLocationTrigger[tr.hostName])
			os.Exit(1)
		}
		if params, ok := categoryConfig.TestLocationTrigger[tr.hostName].TestParameters[tr.testTrigger]; !ok {
			fmt.Printf("failed to get test trigger %+v under category %+v config: %+v\n", tr.testTrigger, categoryConfig, categoryConfig.TestLocationTrigger[tr.hostName])
			os.Exit(1)
		} else if len(params.TestCases) == 0 {
			fmt.Printf("failed to get test case under category %+v trigger %+v config: %+v\n", categoryConfig, tr.testTrigger, categoryConfig.TestLocationTrigger[tr.hostName])
			os.Exit(1)
		}
		tr.testLocation = tr.hostName
		return
	}
	// if host specific config was not found
	// validate global config's trigger
	if categoryConfig.TestLocationTrigger[globals.GlobalTestTriggerKeyword].TestParameters == nil {
		fmt.Printf("failed to get any test trigger under category %+v global config: %+v\n", categoryConfig, categoryConfig.TestLocationTrigger[tr.hostName])
		os.Exit(1)
	}
	if params, ok := categoryConfig.TestLocationTrigger[globals.GlobalTestTriggerKeyword].TestParameters[tr.testTrigger]; !ok {
		fmt.Printf("failed to get test trigger %+v under category %+v global config: %+v\n", tr.testTrigger, categoryConfig, categoryConfig.TestLocationTrigger[tr.hostName])
		os.Exit(1)
	} else if len(params.TestCases) == 0 {
		fmt.Printf("failed to get test case under category %+v trigger %+v global config: %+v\n", categoryConfig, tr.testTrigger, categoryConfig.TestLocationTrigger[tr.hostName])
		os.Exit(1)
	}
	tr.testLocation = globals.GlobalTestTriggerKeyword
}

func (tr *TestRunner) initLogger() {
	logger.SetLogDir(tr.logDir)
	logger.SetLogFile(globals.DefaultRunnerLogSubPath)
	logger.SetLogPrefix(globals.LogPrefix)
	logger.Init(utils.IsKubernetes())
}

// readTestRunnerConfig try to user provided customized test runner config from given file
func (tr *TestRunner) readTestRunnerConfig(configPath string) {
	tr.Lock()
	defer tr.Unlock()

	defer func() {
		tr.normalizeConfig()
	}()

	file, err := os.Open(configPath)
	if err != nil {
		tr.globalTestRunnerConfig = defaultGlobalTestRunnerConfig
		logger.Log.Printf("cannot read provided test runner config at %+v, err: %+v, using default test runner config", configPath, err)
		return
	}
	defer file.Close()
	bytes, err := io.ReadAll(file)
	if err != nil {
		tr.globalTestRunnerConfig = defaultGlobalTestRunnerConfig
		logger.Log.Printf("cannot read provided test runner config at %+v, err: %+v, using default test runner config", configPath, err)
		return
	}
	var config testrunnerGen.TestRunnerConfig
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		tr.globalTestRunnerConfig = defaultGlobalTestRunnerConfig
		logger.Log.Printf("cannot read provided test runner config at %+v, err: %+v, using default test runner config", configPath, err)
		return
	}
	tr.globalTestRunnerConfig = &config
}

func (tr *TestRunner) initTestRunnerConfig() {
	if tr.logDir == "" {
		tr.logDir = globals.DefaultRunnerLogDir
	}

	// init test runner log
	err := os.MkdirAll(tr.logDir, 0755)
	if err != nil {
		fmt.Printf("Failed to create dir for test runner logs %+v, err: %+v\n", tr.logDir, err)
		os.Exit(1)
	}

	// init status db
	// don't try to create if status db already exists
	// test runner needs to read the existing db and rerun incomplete test before crash/restart
	statusDBPath := filepath.Join(tr.logDir, globals.DefaultStatusDBSubPath)
	if _, err := os.Stat(statusDBPath); err != nil && os.IsNotExist(err) {
		_, err = os.Create(statusDBPath)
		if err != nil {
			fmt.Printf("Failed to create test status db %+v, err: %+v\n", statusDBPath, err)
			os.Exit(1)
		}
		runnerStatus := &testrunnerGen.TestRunnerStatus{
			TestStatus: map[string]string{},
		}
		err = SaveRunnerStatus(runnerStatus, statusDBPath)
		if err != nil {
			fmt.Printf("Failed to init test runner status db %+v, err: %+v\n", statusDBPath, err)
			os.Exit(1)
		}
	}
	tr.statusDBPath = statusDBPath

	// the validation of globalTestRunnerConfig has been done previously by validateTestTrigger()
	testParams := tr.getTestParameters()

	gpuModelDir, err := getGPUModelTestRecipeDir(tr.rocmSMIPath)
	if err != nil {
		logger.Log.Printf("failed to get GPU model specific folder for test recipe err %+v, using recipe from root conf folder", err)
	}
	testCfgPath := filepath.Join(tr.rvsTestCaseDir, testParams.TestCases[0].Recipe+".conf")
	if gpuModelDir != "" {
		logger.Log.Printf("using test recipe from %+v folder", gpuModelDir)
		testCfgPath = filepath.Join(tr.rvsTestCaseDir, gpuModelDir, testParams.TestCases[0].Recipe+".conf")
	}
	if _, err := os.Stat(testCfgPath); err != nil {
		fmt.Printf("Trigger %+v cannot find corresponding test config file %+v, err: %+v\n", tr.testTrigger, testCfgPath, err)
		os.Exit(1)
	}
	logger.Log.Printf("applied test config for %+v", tr.testLocation)
}

// the validation functions have make sure that the given category/location/trigger config exists and valid within runnerConfig
// this function will be responsible to trigger the test
func (tr *TestRunner) TriggerTest() {
	switch tr.testCategory {
	case testrunnerGen.TestCategory_GPU_HEALTH_CHECK.String():
		switch tr.testTrigger {
		case testrunnerGen.TestTrigger_AUTO_UNHEALTHY_GPU_WATCH.String():
			// init rvs test runner
			// and start to listen for unix socket to receive the event
			// for triggering the test run on unhealthy GPU
			rvsTestRunner, err := NewRvsTestRunner(tr.rvsPath, tr.rvsTestCaseDir, tr.logDir)
			if err != nil || rvsTestRunner == nil {
				logger.Log.Printf("failed to create rvs test runner, runner: %+v, err: %+v", rvsTestRunner, err)
				os.Exit(1)
			}
			tr.rvsTestRunner = rvsTestRunner
			tr.watchGPUState()
		case testrunnerGen.TestTrigger_MANUAL.String(),
			testrunnerGen.TestTrigger_PRE_START_JOB_CHECK.String():
			rvsTestRunner, err := NewRvsTestRunner(tr.rvsPath, tr.rvsTestCaseDir, tr.logDir)
			if err != nil || rvsTestRunner == nil {
				logger.Log.Printf("failed to create rvs test runner, runner: %+v, err: %+v", rvsTestRunner, err)
				os.Exit(1)
			}
			tr.rvsTestRunner = rvsTestRunner
			tr.manualTestGPU()
		default:
			logger.Log.Printf("unsupported test trigger %+v for category %+v", tr.testTrigger, tr.testCategory)
			os.Exit(1)
		}
	}
}

func (tr *TestRunner) watchGPUState() {
	ticker := time.NewTicker(globals.GPUStateConnRetryFreq)
	defer ticker.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), globals.GPUStateConnREtryTimeout)
	defer cancel()

	var err error
	var conn *grpc.ClientConn
	connected := false

	for !connected {
		select {
		case <-ticker.C:
			conn, err = grpc.NewClient("unix:"+tr.exporterSocketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				logger.Log.Printf("testrunner cannot connect to %v: %v", "unix:"+tr.exporterSocketPath, err)
				continue
			}
			connected = true
			defer conn.Close()
		case <-ctx.Done():
			logger.Log.Fatalf("retry exhausted: testrunner cannot connect to %v", "unix:"+tr.exporterSocketPath)
			return
		}
	}

	c := metricssvc.NewMetricsServiceClient(conn)
	watchTicker := time.NewTicker(globals.GPUStateWatchFreq)
	defer watchTicker.Stop()

	// handle test runner crash or restart
	// read existing test runner status db
	// immediately start test on interrupted test before restarting
	statusObj, _ := LoadRunnerStatus(tr.statusDBPath)
	ids := []string{}
	if statusObj != nil && len(statusObj.TestStatus) > 0 {
		for deviceID, status := range statusObj.TestStatus {
			if status == types.TestRunning.String() {
				ids = append(ids, deviceID)
			}
		}
		logger.Log.Printf("found GPU %+v with incomplete test before restart %+v, start to rerun test", ids, statusObj)
		go tr.testGPU(testrunnerGen.TestTrigger_AUTO_UNHEALTHY_GPU_WATCH.String(), ids, true)
	}

	go tr.watchConfigFile()
	for {
		select {
		case <-watchTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), globals.GPUStateReqTimeout)
			r, err := c.List(ctx, &emptypb.Empty{})
			if err != nil {
				logger.Log.Printf("could not list GPU state: %v", err)
				cancel()
				continue
			}
			logger.Log.Printf("GPU State: %s", r.String())
			cancel()

			healthyGPUIDs := []string{}
			unHealthyGPUIDs := []string{}
			if r != nil {
				for _, state := range r.GPUState {
					// TODO: currently exporter with gpuagent just returns GPU index number
					// we need to convert it to GUID per rvs's request
					// modify this after rvs starts to accept index number as ID
					id, err := GetGUIDFromIndex(state.ID, tr.rocmSMIPath)
					if err != nil {
						logger.Log.Printf("failed to fetch GUID for GPU card%v, err: %+v", state.ID, err)
						continue
					}
					// if any GPU is not healthy, start a test against those GPUs
					if !strings.EqualFold(state.Health, metricssvc.GPUHealth_HEALTHY.String()) {
						if len(state.AssociatedWorkload) == 0 {
							unHealthyGPUIDs = append(unHealthyGPUIDs, id)
						} else {
							logger.Log.Printf("found GPU %+v unhealthy but still associated with workload %+v", id, state.AssociatedWorkload)
						}
					} else {
						healthyGPUIDs = append(healthyGPUIDs, id)
					}
				}
			}

			// start test on unhealthy GPU
			if len(unHealthyGPUIDs) > 0 {
				logger.Log.Printf("found GPU with unhealthy state %+v", unHealthyGPUIDs)
				go tr.testGPU(testrunnerGen.TestTrigger_AUTO_UNHEALTHY_GPU_WATCH.String(), unHealthyGPUIDs, false)
			} else {
				logger.Log.Printf("all GPUs are healthy, skip testing")
			}

			tr.cleanupHealthyGPUTestStatus(healthyGPUIDs)
		}
	}
}

func (tr *TestRunner) watchConfigFile() {
	// if config file doesn't exist, create dir in case it doesn't exist
	// so that fsnotify file watcher won't fail to init the watcher
	directory := path.Dir(tr.testCfgPath)
	os.MkdirAll(directory, 0755)
	logger.Log.Printf("starting file watcher for %v", directory)

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Log.Fatal(err)
	}
	defer watcher.Close()
	ctx := context.Background()
	// Start listening for events.
	go func() {
		for ctx.Err() == nil {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// k8s has to many cases to handle because of symlink, to be
				// safe handle all cases
				if event.Has(fsnotify.Create | fsnotify.Write | fsnotify.Remove | fsnotify.Rename) {
					logger.Log.Printf("loading new config on %v", tr.testCfgPath)
					tr.readTestRunnerConfig(tr.testCfgPath)
					tr.validateTestTrigger()
					logger.Log.Printf("Test runner isKubernetes: %+v config: %+v", tr.isK8s, tr.globalTestRunnerConfig)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					logger.Log.Printf("error watching for config file: %v", err)
					return
				}
			}
		}
	}()

	// Add a path.
	err = watcher.Add(directory)
	if err != nil {
		logger.Log.Printf("failed to start the config file watcher err %+v", err)
		log.Fatal(err)
	}

	<-make(chan struct{})
}

func (tr *TestRunner) cleanupHealthyGPUTestStatus(ids []string) {
	// for healthy GPU
	// check if there is test status cached
	// 1. if there is test already running
	// don't interrupt the running test
	// 2. if there is test completed
	// remove the status so that next time it turns unhealthy, test will be triggered again
	statusObj, _ := LoadRunnerStatus(tr.statusDBPath)
	writeBack := false
	if statusObj != nil && statusObj.TestStatus != nil {
		for _, healthyID := range ids {
			if status, ok := statusObj.TestStatus[healthyID]; ok && status != types.TestRunning.String() {
				delete(statusObj.TestStatus, healthyID)
				writeBack = true
			}
		}
	} else {
		statusObj = &testrunnerGen.TestRunnerStatus{}
		writeBack = true
	}
	if writeBack {
		SaveRunnerStatus(statusObj, tr.statusDBPath)
	}
}

func (tr *TestRunner) testGPU(trigger string, ids []string, isRerun bool) {
	parameters := tr.getTestParameters()
	// load ongoing test status
	// avoid run multiple test on the same device
	validIDs, statusObj := removeIDsWithExistingTest(trigger, tr.statusDBPath, ids, parameters, isRerun)
	if isRerun {
		// for rerun after test runner restart
		// we need to force to run the incomplete test
		// ignore the status db temporarily
		validIDs = ids
	}
	if len(ids) > 0 && len(validIDs) == 0 {
		// all original target devices have existing running test, skip for now
		return
	}
	// if both len(ids) and len(validIDs) are 0
	// that means all devices were selected

	handler, err := tr.rvsTestRunner.GetTestHandler(parameters.TestCases[0].Recipe, types.TestParams{
		Iterations:    uint(parameters.TestCases[0].Iterations),
		StopOnFailure: parameters.TestCases[0].StopOnFailure,
		DeviceIDs:     validIDs,
		Timeout:       uint(parameters.TestCases[0].TimeoutSeconds),
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

	tr.AddTestRunningLabel(parameters.TestCases[0].Recipe)
	defer tr.RemoveTestRunningLabel(parameters.TestCases[0].Recipe)

	if len(validIDs) == 0 {
		// all devices were selected
		ids, err := GetAllGUIDs(tr.rocmSMIPath)
		if err != nil {
			// TODO: add more error handling when failed to get all GUIDs
		}
		validIDs = ids
	}
	for _, id := range validIDs {
		statusObj.TestStatus[id] = types.TestRunning.String()
	}

	err = SaveRunnerStatus(statusObj, tr.statusDBPath)
	if err != nil {
		//TODO: add error handling here if new running status failed to be saved
	}

	select {
	case <-time.After(time.Duration(parameters.TestCases[0].TimeoutSeconds) * time.Second):
		logger.Log.Printf("Trigger: %v Test: %v GPU IDs: %v timeout", trigger, parameters.TestCases[0].Recipe, ids)
		result := BuildTimedoutTestSummary(validIDs)
		tr.generateK8sEvent(parameters.TestCases[0].Recipe, v1.EventTypeWarning, testrunnerGen.TestEventReason_TestTimedOut.String(), result)
		// save log into gzip file
		if stdout := handler.Stdout(); stdout != "" {
			SaveTestResultToGz(stdout, GetLogFilePath(tr.logDir, trigger, parameters.TestCases[0].Recipe, "stdout"))
		}
		if stderr := handler.Stderr(); stderr != "" {
			SaveTestResultToGz(stderr, GetLogFilePath(tr.logDir, trigger, parameters.TestCases[0].Recipe, "stderr"))
		}
		handler.StopTest()
		// exit on non-auto trigger's failure
		tr.exitOnFailure(result)
	case <-handler.Done():
		// TODO: this has to change later based on result logs parsing.
		// for now updating same result in all GPU
		result := handler.Result()
		logger.Log.Printf("Trigger: %v Test: %v GPU IDs: %v completed. Result: %v", trigger, parameters.TestCases[0].Recipe, ids, result)
		// save log into gzip file
		if stdout := handler.Stdout(); stdout != "" {
			SaveTestResultToGz(stdout, GetLogFilePath(tr.logDir, trigger, parameters.TestCases[0].Recipe, "stdout"))
		}
		if stderr := handler.Stderr(); stderr != "" {
			SaveTestResultToGz(stderr, GetLogFilePath(tr.logDir, trigger, parameters.TestCases[0].Recipe, "stderr"))
		}
		tr.generateK8sEvent(parameters.TestCases[0].Recipe, v1.EventTypeNormal, testrunnerGen.TestEventReason_TestPassed.String(), result)
		// exit on non-auto trigger's failure
		tr.exitOnFailure(result)
	}

	statusObj, _ = LoadRunnerStatus(tr.statusDBPath)
	for _, id := range validIDs {
		switch tr.testTrigger {
		case testrunnerGen.TestTrigger_MANUAL.String(),
			testrunnerGen.TestTrigger_PRE_START_JOB_CHECK.String():
			// the status db is for internal usage only
			// for MANUAL and PRE_START_JOB_CHECK test trigger
			// remove the device id from status db once the test was completed
			// so that the next time the device won't be recognized with incomplete test
			delete(statusObj.TestStatus, id)
		case testrunnerGen.TestTrigger_AUTO_UNHEALTHY_GPU_WATCH.String():
			// the status db is for internal usage only
			// for AUTO_UNHEALTHY_GPU_WATCH just mark all finished test as completed
			// so that there won't be another test happened on the same unhealthy device
			// the test completed status will be removed if device becomes healthy again
			statusObj.TestStatus[id] = types.TestCompleted.String()
		}
	}
	SaveRunnerStatus(statusObj, tr.statusDBPath)
}

func (tr *TestRunner) exitOnFailure(result map[string]map[string]types.TestResult) {
	switch tr.testTrigger {
	case testrunnerGen.TestTrigger_MANUAL.String(),
		testrunnerGen.TestTrigger_PRE_START_JOB_CHECK.String():
		if result == nil {
			logger.Log.Printf("failed to get result, exiting...")
			os.Exit(1)
		}
		for guid, actionResults := range result {
			for action, result := range actionResults {
				switch result {
				case types.Failure, types.Cancelled, types.Timedout:
					logger.Log.Printf("error GPU %+v test action %+v failed due to %+v", guid, action, result)
					os.Exit(1)
				}
			}
		}
	}
}

func (tr *TestRunner) manualTestGPU() {
	// for manual test
	// if there is no GPU detected, fail the test runner process
	allGUIDs, err := GetAllGUIDs(tr.rocmSMIPath)
	if err != nil {
		logger.Log.Printf("failed to detect GPU by rocm-smi err %+v", err)
		os.Exit(1)
	}
	if len(allGUIDs) == 0 {
		logger.Log.Println("no GPU was detected by rocm-smi")
		result := BuildNoGPUTestSummary()
		parameters := tr.getTestParameters()
		tr.generateK8sEvent(parameters.TestCases[0].Recipe, v1.EventTypeWarning, testrunnerGen.TestEventReason_TestFailed.String(), result)
		// exit on non-auto trigger's failure
		tr.exitOnFailure(result)
	}

	// handle test runner crash or restart
	// read existing test runner status db
	// immediately start test on interrupted test before restarting
	statusObj, _ := LoadRunnerStatus(tr.statusDBPath)
	ids := []string{}
	if statusObj != nil && len(statusObj.TestStatus) > 0 {
		for deviceID := range statusObj.TestStatus {
			ids = append(ids, deviceID)
		}
		logger.Log.Printf("found GPU %+v with incomplete test before restart %+v, start to rerun test", ids, statusObj)
		tr.testGPU(tr.testTrigger, ids, true)
	} else {
		tr.testGPU(tr.testTrigger, ids, false)
	}
}

func (tr *TestRunner) ReadPodInfo() {
	if tr.k8sPodName == "" {
		tr.k8sPodName = os.Getenv("POD_NAME")
	}
	if tr.k8sPodNamespace == "" {
		tr.k8sPodNamespace = os.Getenv("POD_NAMESPACE")
	}
}

func (tr *TestRunner) AddTestRunningLabel(recipe string) {
	if !tr.isK8s {
		return
	}
	key, val := GetTestRunningLabelKeyValue(tr.testCategory, recipe)
	tr.k8sClient.AddNodeLabel(tr.hostName, key, val)
}

func (tr *TestRunner) RemoveTestRunningLabel(recipe string) {
	if !tr.isK8s {
		return
	}
	key, _ := GetTestRunningLabelKeyValue(tr.testCategory, recipe)
	tr.k8sClient.RemoveNodeLabel(tr.hostName, key)
}

func (tr *TestRunner) normalizeConfig() {
	// convert category to uppercase so that config map won't be case sensitive
	if tr.globalTestRunnerConfig != nil {
		newConfigMap := map[string]*testrunnerGen.TestCategoryConfig{}
		for category, categoryConfig := range tr.globalTestRunnerConfig.TestConfig {
			if categoryConfig != nil {
				newConfigMap[strings.ToUpper(category)] = categoryConfig
				newLocationConfig := map[string]*testrunnerGen.TestTriggerConfig{}
				for location, triggerConfig := range categoryConfig.TestLocationTrigger {
					if triggerConfig != nil {
						newParams := map[string]*testrunnerGen.TestParameters{}
						for trigger, params := range triggerConfig.TestParameters {
							newParams[strings.ToUpper(trigger)] = params
						}
						newLocationConfig[location] = &testrunnerGen.TestTriggerConfig{
							TestParameters: newParams,
						}
					}
				}
				categoryConfig.TestLocationTrigger = newLocationConfig
			}
		}
		tr.globalTestRunnerConfig.TestConfig = newConfigMap
	}
}

func (tr *TestRunner) getTestParameters() *testrunnerGen.TestParameters {
	tr.Lock()
	defer tr.Unlock()
	return tr.globalTestRunnerConfig.TestConfig[tr.testCategory].TestLocationTrigger[tr.testLocation].TestParameters[tr.testTrigger]
}

func (tr *TestRunner) getHostName() {
	hostName, err := os.Hostname()
	if err != nil {
		logger.Log.Printf("failed to get hostname, err: %+v", err)
	}
	tr.hostName = hostName
	if utils.IsKubernetes() {
		tr.hostName = os.Getenv("NODE_NAME")
	}
	logger.Log.Printf("HostName: %v", tr.hostName)
}

func (tr *TestRunner) generateK8sEvent(testRecipe, evtType, reason string, summary map[string]map[string]types.TestResult) {
	if !tr.isK8s {
		// return if it is not running in k8s cluster
		return
	}
	tr.ReadPodInfo()
	if tr.k8sPodName == "" || tr.k8sPodNamespace == "" {
		logger.Log.Printf("failed to get pod name or pod namespace: name: %+v namespace: %+v, skip generating event for recipe %+v evtType %+v reason %+v summary %+v",
			tr.k8sPodName, tr.k8sPodNamespace, testRecipe, evtType, reason, summary)
		return
	}

	msg, err := json.Marshal(summary)
	if err != nil {
		logger.Log.Panicf("failed to marshal test summary %+v err %+v", summary, err)
		return
	}
	evtNamePrefix := GetEventName(tr.testCategory, tr.testTrigger, testRecipe)
	// if there is no event exist, create a new one
	currTime := time.Now().UTC()
	evtObj := &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: evtNamePrefix,
			Namespace:    tr.k8sPodNamespace,
		},
		FirstTimestamp: metav1.Time{
			Time: currTime,
		},
		LastTimestamp: metav1.Time{
			Time: currTime,
		},
		Count:   1,
		Type:    evtType,
		Reason:  reason,
		Message: string(msg),
		InvolvedObject: v1.ObjectReference{
			Kind:      "Pod",
			Namespace: tr.k8sPodNamespace,
			Name:      tr.k8sPodName,
		},
		Source: v1.EventSource{
			Host:      tr.hostName,
			Component: globals.EventSourceComponentName,
		},
	}
	// TODO: handle error for failing to generate event
	tr.k8sClient.CreateEvent(evtObj)
}
