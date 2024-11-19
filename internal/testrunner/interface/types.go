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

package types

import (
	"bytes"
	"context"
	"log"
	"os/exec"
	"sync"
	"time"
)

// DefaultTestTimeout is default test timeout
const DefaultTestTimeout = 600 // 10 min

// TOption fills the optional params for Test Handler
type TOption func(*TestHandler)

// TestWithTimeout passes a timeout to the Test handler
func TestWithTimeout(timeout uint) TOption {
	return func(th *TestHandler) {
		th.timeout = timeout
	}
}

// TestWithLogFilePath sets the log file for the current test execution
func TestWithLogFilePath(logFilePath string) TOption {
	return func(th *TestHandler) {
		th.logFilePath = logFilePath
	}
}

// TestHandler runs a given test CLI
type TestHandler struct {
	testname    string
	args        []string
	process     *exec.Cmd
	stdout      bytes.Buffer
	stderr      bytes.Buffer
	cancelFunc  context.CancelFunc
	wg          sync.WaitGroup
	timeout     uint
	logFilePath string
	logger      *log.Logger
	status      CommandStatus
	rwLock      sync.RWMutex
	result      map[string]TestResult // component id -> result
	doneChan    chan struct{}
}

// NewTestHandler returns instance of TestHandler
func NewTestHandler(testname string, logger *log.Logger, args []string, opts ...TOption) TestHandlerInterface {
	hldr := &TestHandler{
		testname: testname,
		args:     args,
		wg:       sync.WaitGroup{},
		logger:   logger,
		timeout:  DefaultTestTimeout,
		status:   TestNotStarted,
		rwLock:   sync.RWMutex{},
		doneChan: make(chan struct{}),
	}

	for _, o := range opts {
		o(hldr)
	}

	return hldr
}

// StartTest starts the CLI execution
func (th *TestHandler) StartTest() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(th.timeout)*time.Second)
	th.process = exec.CommandContext(ctx, th.args[0], th.args[1:]...)
	th.process.Stdout = &th.stdout
	th.process.Stderr = &th.stderr
	th.cancelFunc = cancel

	if err := th.process.Start(); err != nil {
		return err
	}
	th.setStatus(TestRunning)
	th.logger.Printf("cmd %v [pid=%v] started running", th.testname, th.process.Process.Pid)
	th.wg.Add(1)
	go func() {
		defer th.wg.Done()
		err := th.process.Wait()
		th.setStatus(TestCompleted)
		th.logger.Printf("cmd %v [pid=%v] completed", th.testname, th.process.Process.Pid)
		if err != nil {
			th.logger.Printf("cmd %v [pid=%v] has exited with error %v", th.testname, th.process.Process.Pid, err)
		}
		th.doneChan <- struct{}{}
		th.cancelCtx()
	}()
	return nil
}

// StopTest stops the current test execution
func (th *TestHandler) StopTest() {
	th.logger.Printf("stop test called for %v [pid=%v]", th.testname, th.process.Process.Pid)
	th.cancelCtx()
	th.wg.Wait()
}

// cancelCtx calls the cancel function
func (th *TestHandler) cancelCtx() {
	if th.cancelFunc == nil {
		return
	}
	th.cancelFunc()
	th.cancelFunc = nil
}

// Stdout return stdout of the test command
func (th *TestHandler) Stdout() string {
	return th.stdout.String()
}

// Stderr return stderr of the test command
func (th *TestHandler) Stderr() string {
	return th.stderr.String()
}

// GetLogFilePath return log file path of the test command
func (th *TestHandler) GetLogFilePath() string {
	return th.logFilePath
}

// Status returns status of the test command
func (th *TestHandler) Status() CommandStatus {
	th.rwLock.RLock()
	defer th.rwLock.RUnlock()
	return th.status
}

// Result for the test
func (th *TestHandler) Result() map[string]TestResult {
	// TODO, set the result after parsing the logs
	th.logger.Printf("stdout: %+v", th.stdout.String())
	th.logger.Printf("stderr: %+v", th.stderr.String())
	return th.result
}

// Done is used to signal completion of the test
func (th *TestHandler) Done() chan struct{} {
	return th.doneChan
}

func (th *TestHandler) setStatus(status CommandStatus) {
	th.rwLock.Lock()
	defer th.rwLock.Unlock()
	th.status = status
}
