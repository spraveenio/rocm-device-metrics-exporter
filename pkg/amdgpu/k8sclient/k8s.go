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

package k8sclient

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/logger"
	"github.com/pensando/device-metrics-exporter/pkg/amdgpu/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	//
	// Uncomment to load all auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth"
	//
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

type K8sClient struct {
	sync.Mutex
	clientset *kubernetes.Clientset
}

func NewClient() *K8sClient {
	return &K8sClient{}
}

func (k *K8sClient) init() error {
	k.Lock()
	defer k.Unlock()

	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Log.Printf("k8s cluster config error %v", err)
		return err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Log.Printf("clientset from config failed %v", err)
		return err
	}

	k.clientset = clientset
	return nil
}

func (k *K8sClient) reConnect() error {
	if k.clientset == nil {
		return k.init()
	}
	return nil
}

func (k *K8sClient) GetNodelLabel(nodeName string) (string, error) {
	k.reConnect()
	k.Lock()
	defer k.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	node, err := k.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		logger.Log.Printf("k8s internal node get failed %v", err)
		k.clientset = nil
		return "", err
	}
	return fmt.Sprintf("%+v", node.Labels), nil
}

func (k *K8sClient) UpdateHealthLabel(nodeName string, newHealthMap map[string]string) error {
	k.reConnect()
	k.Lock()
	defer k.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	node, err := k.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		logger.Log.Printf("k8s internal node get failed %v", err)
		k.clientset = nil
		return err
	}

	oldHealthMap := utils.ParseNodeHealthLabel(node.Labels)

	// check diff
	if reflect.DeepEqual(oldHealthMap, newHealthMap) {
		// logger.Log.Printf("ignoring update no change on label values")
		return nil
	}
	utils.RemoveNodeHealthLabel(node.Labels)
	utils.AddNodeHealthLabel(node.Labels, newHealthMap)

	logger.Log.Printf("Updating node health labels %+v", node.Labels)

	// Update the node
	_, err = k.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		logger.Log.Printf("k8s internal node update failed %v", err)
		k.clientset = nil
		return err
	}

	return nil
}
