# Troubleshooting Device Metrics Exporter

This topic provides an overview of troubleshooting options for Device Metrics Exporter.

## Using Techsupport-dump Tool

The [techsupport-dump script](https://github.com/ROCm/device-metrics-exporter/blob/main/tools/techsupport_dump.sh) can be used to collect system state and logs for debugging:

```bash
# ./techsupport_dump.sh [-w] [-o yaml/json] [-k kubeconfig] [-r helm-release-name] <node-name/all>
```

Options:

- `-w`: wide option
- `-o yaml/json`: output format (default: json)
- `-k kubeconfig`: path to kubeconfig (default: ~/.kube/config)
- `-r  helm-release-name`: helm release name

Please file an issue with collected techsupport bundle on our [GitHub Issues](https://github.com/ROCm/device-metrics-exporter/issues) page

## Logs
You can view the container logs by executing the following command:

### Docker deployment

```bash
docker logs device-metrics-exporter
```

### K8s deployment
```bash
kubectl logs -n <namespace> <exporter-container-on-node>
```

### Debian deployment

```bash
sudo journalctl -xu amd-metrics-exporter
```

logs are collected in directory `/var/log/` files 
- exporter.log
- gpu-agent.log
- gpu-agent-api.log
- gpu-agent-err.log

#### Debian Techsupport Collection Command

```bash
sudo journalctl -xu amd-metrics-exporter > amd-metrics-exporter.log
sudo journalctl -xu gpuagent > amd-gpu-agent.log

sudo tar -czf amd-metrics-exporter-techsupport-$(date +%Y%m%d-%H%M%S).tar.gz \
   amd-metrics-exporter.log \
   amd-gpu-agent.log \
   /var/log/exporter.log \
   /var/log/gpu-agent*.log

```

## Common Issues

This section describes common issues with AMD Device Metrics Exporter

1. Port conflicts:
   - Verify port 5000 is available
   - Configure an alternate port through the configuration file

2. Device access:
   - Ensure proper permissions on `/dev/dri` and `/dev/kfd`
   - Verify ROCm is properly installed

3. Metric collection issues:
   - Check GPU driver status
   - Verify ROCm version compatibility
