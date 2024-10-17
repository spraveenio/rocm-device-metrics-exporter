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

package slurm

import (
	"context"
	"encoding/json"
	"fmt"
	zmq "github.com/go-zeromq/zmq4"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/gen/gpumetrics"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/gen/luaplugin"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/globals"
	"github.com/pensando/device-metrics-exporter/internal/amdgpu/logger"
	"google.golang.org/protobuf/proto"
	"io"
	"net"
	"os"
	"path"
	"strings"
	"sync"
)

var jobLabels = map[string]bool{
	gpumetrics.GPUMetricLabel_JOB_ID.String(): true,
}

type JobsService interface {
	ListJobs() map[string]JobInfo
	CheckExportLabels(labels map[string]bool) bool
	Close() error
}
type JobInfo struct {
	JobId string
}
type client struct {
	sync.Mutex
	zmqSock   zmq.Socket
	slurmSock net.Listener
	GpuJobs   map[string]JobInfo
}

func NewClient(ctx context.Context) (JobsService, error) {
	sock := zmq.NewPull(ctx)
	logger.Log.Printf("Starting Listen on port %v", globals.ZmqPort)
	if err := sock.Listen(fmt.Sprintf("tcp://*:%v", globals.ZmqPort)); err != nil {
		return nil, fmt.Errorf("failed to listen on port %v, %v ", globals.ZmqPort, err)
	}

	os.MkdirAll(path.Dir(globals.SlurmSock), 0644)
	os.Remove(globals.SlurmSock)
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: globals.SlurmSock, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("failed to listen on  %v, %v ", globals.SlurmSock, err)
	}

	cl := &client{
		zmqSock:   sock,
		slurmSock: l,
		GpuJobs:   make(map[string]JobInfo),
	}

	var slurmMsg luaplugin.Notification

	go func() {
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				logger.Log.Printf("context done")
			default:
				logger.Log.Printf("waiting for job notifications")
				msg, err := sock.Recv()
				if err != nil {
					if err != io.EOF {
						logger.Log.Printf("could not receive message %v", err)
					}
					break
				}
				if err := proto.Unmarshal(msg.Bytes(), &slurmMsg); err != nil {
					logger.Log.Printf("could not receive message %v", err)
					break
				}
				logger.Log.Printf("received slurm notification %+v", slurmMsg.String())
				if slurmMsg.SData == nil {
					logger.Log.Printf("SData is empty %+v", slurmMsg.SData)
					break
				}

				logger.Log.Printf("slurm msg type:%v job:%v gpus:%v", slurmMsg.Type, slurmMsg.SData.JobID, slurmMsg.SData.AllocGPUs)
				switch slurmMsg.Type {
				case luaplugin.Stages_TaskInit:
					if slurmMsg.SData.JobID > 0 && len(slurmMsg.SData.AllocGPUs) > 0 {
						cl.Lock()
						for _, allocGPU := range slurmMsg.SData.AllocGPUs {
							cl.GpuJobs[allocGPU] = JobInfo{
								JobId: fmt.Sprintf("%v", slurmMsg.SData.JobID),
							}

						}
						cl.Unlock()
					}

				case luaplugin.Stages_TaskExit:
					cl.Lock()
					for _, allocGPU := range slurmMsg.SData.AllocGPUs {
						delete(cl.GpuJobs, allocGPU)

					}
					cl.Unlock()
				case luaplugin.Stages_TaskEpilog:
					logger.Log.Printf("ignore msg type %v", slurmMsg.Type)
				default:
					logger.Log.Printf("unknown msg type %v", slurmMsg.Type)
				}
			}
		}
	}()

	go func() {
		defer os.Remove(globals.SlurmSock)
		for {
			for ctx.Err() == nil {
				sfd, err := cl.slurmSock.Accept()
				if err != nil {
					logger.Log.Printf("failed to accept slurm connection %v", err)
					continue
				}
				logger.Log.Printf("new slurm message")
				cl.processSlurm(ctx, sfd)
			}
		}
	}()
	return cl, nil
}

func (cl *client) processSlurm(ctx context.Context, sfd net.Conn) {
	defer sfd.Close()

	for {
		select {
		case <-ctx.Done():
			logger.Log.Printf("context done")
		default:
			buff := make([]byte, 4096)
			len, err := sfd.Read(buff)
			if err != nil {
				if err != io.EOF {
					logger.Log.Printf("could not read message %v", err)
				}
				return
			}
			var jobEnv map[string]string
			if err := json.Unmarshal(buff[:len], &jobEnv); err != nil {
				logger.Log.Printf("could not parse job env %v", err)
				logger.Log.Printf("job env %v ", string(buff))
				break
			}
			logger.Log.Printf("received job env %+v", jobEnv)
			switch jobEnv["SLURM_SCRIPT_CONTEXT"] {
			case "prolog_slurmd":
				if gpus, ok := jobEnv["CUDA_VISIBLE_DEVICES"]; ok {
					cl.Lock()
					for _, allocGPU := range strings.Split(gpus, ",") {
						cl.GpuJobs[allocGPU] = JobInfo{
							JobId: fmt.Sprintf("%v", jobEnv["SLURM_JOBID"]),
						}
					}
					cl.Unlock()
					logger.Log.Printf("updated %v", cl.GpuJobs)
				}

			case "epilog_slurmd":
				if gpus, ok := jobEnv["CUDA_VISIBLE_DEVICES"]; ok {
					cl.Lock()
					for _, allocGPU := range strings.Split(gpus, ",") {
						delete(cl.GpuJobs, allocGPU)
					}
					cl.Unlock()
					logger.Log.Printf("updated %v", cl.GpuJobs)
				}
			}
		}
	}
}

func (cl *client) ListJobs() map[string]JobInfo {
	jobs := make(map[string]JobInfo)
	cl.Lock()
	defer cl.Unlock()
	for k, v := range cl.GpuJobs {
		jobs[k] = v
	}
	return jobs
}
func (cl *client) CheckExportLabels(labels map[string]bool) bool {
	for k := range jobLabels {
		if ok := labels[k]; ok {
			return true
		}
	}
	return false
}
func (cl *client) Close() error {
	cl.zmqSock.Close()
	cl.slurmSock.Close()
	return nil
}
