internal testing tool usage and notes

exporter is packed with basic set of tools for debugging and testing some of
the workflows mocking data

#metricsclient -h
Usage of metricsclient:
  -ecc-file-path string
        json ecc err file
  -get
        send get req
  -id string
        send gpu id (default "1")
  -json
        output in json format
  -label
        get k8s node label
  -socket string
        metrics grpc socket path (default
        "/sockets/amdgpu_device_metrics_exporter_grpc.socket")

===========================================================================
1. list health status of gpus reported by the exporter

[root@e2e-test-k8s-amdgpu-metrics-exporter-n8lvh ~]# metricsclient
ID      Health  Associated Workload
------------------------------------------------
0       healthy []
------------------------------------------------
[root@e2e-test-k8s-amdgpu-metrics-exporter-n8lvh ~]#
===========================================================================

2. To simulate ecc error create a json file of the below format with gpu id, the
   fields set to ecc fields and counts to respective fields to be updated and issue the below command. 
  This will print the previous reported health status of the exporter and set of counters mocked

[root@e2e-test-k8s-amdgpu-metrics-exporter-n8lvh ~]# cat ecc.json
{
        "ID": "0",
        "Fields": [
                "GPU_ECC_UNCORRECT_SEM",
                "GPU_ECC_UNCORRECT_FUSE"
        ],
        "Counts" : [
                1, 2
        ]
}
[root@e2e-test-k8s-amdgpu-metrics-exporter-n8lvh ~]#
[root@e2e-test-k8s-amdgpu-metrics-exporter-n8lvh ~]# metricsclient -ecc-file-path ecc.json
ID      Health  Associated Workload
------------------------------------------------
0       healthy []
------------------------------------------------
{"ID":"0","Fields":["GPU_ECC_UNCORRECT_SEM","GPU_ECC_UNCORRECT_FUSE"]}
[root@e2e-test-k8s-amdgpu-metrics-exporter-n8lvh ~]#

3. To remove mock fields set the respective field count values to 0 on the json file

[root@e2e-test-k8s-amdgpu-metrics-exporter-n8lvh ~]# cat ecc_delete.json
{
        "ID": "0",
        "Fields": [
                "GPU_ECC_UNCORRECT_SEM",
                "GPU_ECC_UNCORRECT_FUSE"
        ],
        "Counts" : [
                0, 0
        ]
}
[root@e2e-test-k8s-amdgpu-metrics-exporter-n8lvh ~]# metricsclient -ecc-file-path ecc_delete.json
ID      Health  Associated Workload
------------------------------------------------
0       unhealthy       []
------------------------------------------------
{"ID":"0","Fields":["GPU_ECC_UNCORRECT_SEM","GPU_ECC_UNCORRECT_FUSE"]}
[root@e2e-test-k8s-amdgpu-metrics-exporter-n8lvh ~]#






