package k8s

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	kube "k8s.io/kubelet/pkg/apis/podresources/v1alpha1"
	"strings"

	"os"
)

const PodResourceSocket = "/var/lib/kubelet/pod-resources/kubelet.sock"
const amdGpuResourceName = "amd.com/gpu"

type PodResourceInfo struct {
	Pod       string
	Namespace string
	Container string
}

type PodResourcesService interface {
	ListPods(ctx context.Context) (map[string]PodResourceInfo, error)
}

type podResourcesClient struct {
	clientConn *grpc.ClientConn
}

func IsKubernetes() bool {
	if s := os.Getenv("KUBERNETES_SERVICE_HOST"); s != "" {
		return true
	}
	return false
}

func NewClient() (PodResourcesService, error) {
	if _, err := os.Stat(PodResourceSocket); err != nil {
		return nil, fmt.Errorf("no kubelet, %v", err)
	}
	client, err := grpc.NewClient("unix://"+PodResourceSocket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("kubelet socket error, %v", err)
	}
	return &podResourcesClient{clientConn: client}, nil

}

func (pr *podResourcesClient) ListPods(ctx context.Context) (map[string]PodResourceInfo, error) {
	prCl := kube.NewPodResourcesListerClient(pr.clientConn)
	resp, err := prCl.List(ctx, &kube.ListPodResourcesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pod resources, %v", err)
	}

	podInfo := make(map[string]PodResourceInfo)
	for _, pod := range resp.PodResources {
		for _, container := range pod.Containers {
			for _, devs := range container.GetDevices() {
				if devs.ResourceName == amdGpuResourceName {
					for _, devId := range devs.DeviceIds {
						podInfo[strings.ToLower(devId)] = PodResourceInfo{
							Pod:       pod.Name,
							Namespace: pod.Namespace,
							Container: container.Name,
						}
					}
				}
			}
		}
	}
	return podInfo, nil
}
