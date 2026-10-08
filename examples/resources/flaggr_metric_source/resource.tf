resource "flaggr_metric_source" "prometheus" {
  project_id = flaggr_project.platform.id
  name       = "Production Prometheus"
  type       = "prometheus"
  enabled    = true
  config = jsonencode({
    endpoint = "https://prometheus.example.com"
    queries = {
      errorRate  = "sum(rate(http_requests_total{status=~'5..'}[5m]))"
      latencyP99 = "histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))"
      throughput = "sum(rate(http_requests_total[5m]))"
    }
  })
}
