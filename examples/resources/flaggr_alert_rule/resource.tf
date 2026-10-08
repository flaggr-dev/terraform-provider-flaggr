resource "flaggr_alert_rule" "high_error_rate" {
  project_id       = flaggr_project.platform.id
  name             = "High Error Rate"
  description      = "Alert when the error rate exceeds 5%"
  severity         = "critical"
  condition_type   = "error_rate"
  threshold        = 5.0
  window_minutes   = 5
  cooldown_minutes = 15
  channels = jsonencode([
    {
      channelId   = flaggr_alert_channel.slack_ops.id
      channelName = flaggr_alert_channel.slack_ops.name
    }
  ])
}

resource "flaggr_alert_rule" "slow_evaluations" {
  project_id     = flaggr_project.platform.id
  name           = "Slow Evaluations"
  severity       = "warning"
  condition_type = "p95_latency"
  threshold      = 250 # milliseconds
  window_minutes = 10
  channels = jsonencode([
    {
      channelId   = flaggr_alert_channel.slack_ops.id
      channelName = flaggr_alert_channel.slack_ops.name
    }
  ])
}
