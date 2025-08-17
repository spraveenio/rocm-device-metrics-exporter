# Grafana Dashboards

## Dashboards

- `dashboard_overview.json`: High-level GPU cluster overview.
- `dashboard_gpu.json`: Detailed per-GPU metrics.
- `dashboard_job.json`: GPU usage by job (Slurm and Kubernetes).
- `dashboard_node.json`: Host-level GPU usage.

## Variables

Variables can be configured at any time in each dashboard's **Settings > Variables** section.

**g_metrics_prefix**: string to prefix names of metrics queries (e.g. gpu_gfx_activity -> amd_gpu_gfx_activity)
