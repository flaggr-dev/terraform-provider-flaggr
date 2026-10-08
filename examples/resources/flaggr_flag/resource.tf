# default_value is JSON: wrap it in jsonencode() for every flag type.
resource "flaggr_flag" "new_checkout" {
  project_id    = flaggr_project.platform.id
  service_id    = flaggr_service.api.id
  key           = "new-checkout-flow"
  name          = "New Checkout Flow"
  type          = "boolean"
  enabled       = true
  default_value = jsonencode(false)
  environment   = "production"
}

resource "flaggr_flag" "banner_text" {
  project_id    = flaggr_project.platform.id
  service_id    = flaggr_service.api.id
  key           = "banner-text"
  name          = "Banner Text"
  type          = "string"
  enabled       = true
  default_value = jsonencode("Welcome to our platform!")
  environment   = "production"
}

resource "flaggr_flag" "upload_limit" {
  project_id    = flaggr_project.platform.id
  service_id    = flaggr_service.api.id
  key           = "upload-limit-mb"
  name          = "Upload Limit (MB)"
  type          = "number"
  enabled       = true
  default_value = jsonencode(25)
  environment   = "production"
}

resource "flaggr_flag" "checkout_config" {
  project_id  = flaggr_project.platform.id
  service_id  = flaggr_service.api.id
  key         = "checkout-config"
  name        = "Checkout Config"
  description = "Payment provider settings for the checkout"
  type        = "object"
  enabled     = true
  default_value = jsonencode({
    provider = "stripe"
    retries  = 3
  })
  environment = "staging"
}
