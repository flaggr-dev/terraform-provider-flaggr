terraform {
  required_providers {
    flaggr = {
      source  = "flaggr-dev/flaggr"
      version = "~> 0.1"
    }
  }
}

# Both arguments are optional: the provider falls back to the FLAGGR_API_URL
# and FLAGGR_API_TOKEN environment variables.
provider "flaggr" {
  api_url   = "https://flaggr.dev" # the default
  api_token = var.flaggr_api_token
}

variable "flaggr_api_token" {
  description = "Flaggr personal access token (fgp_…, recommended) or project API token (fgr_…)"
  type        = string
  sensitive   = true
}
