variable "slack_webhook_url" {
  type      = string
  sensitive = true
}

variable "pagerduty_routing_key" {
  type      = string
  sensitive = true
}

variable "alert_webhook_secret" {
  type      = string
  sensitive = true
}

resource "flaggr_alert_channel" "slack_ops" {
  project_id = flaggr_project.platform.id
  name       = "Ops Slack"
  type       = "slack"
  enabled    = true
  config = jsonencode({
    webhookUrl = var.slack_webhook_url
  })
}

resource "flaggr_alert_channel" "pagerduty" {
  project_id = flaggr_project.platform.id
  name       = "On-Call PagerDuty"
  type       = "pagerduty"
  config = jsonencode({
    routingKey = var.pagerduty_routing_key
  })
}

resource "flaggr_alert_channel" "email" {
  project_id = flaggr_project.platform.id
  name       = "Release Managers"
  type       = "email"
  config = jsonencode({
    recipients = ["releases@example.com"]
  })
}

resource "flaggr_alert_channel" "webhook" {
  project_id = flaggr_project.platform.id
  name       = "Custom Webhook"
  type       = "webhook"
  config = jsonencode({
    url    = "https://hooks.example.com/flaggr-alerts"
    secret = var.alert_webhook_secret
  })
}
