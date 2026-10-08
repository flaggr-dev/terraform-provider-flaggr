resource "flaggr_environment" "qa" {
  project_id  = flaggr_project.platform.id
  slug        = "qa"
  name        = "QA"
  description = "Pre-release testing"
  color       = "#10b981"
  order       = 30
}

# In a protected environment that requires approval, flag changes go through
# change requests: manage its flags in the Flaggr dashboard (see flaggr_flag).
resource "flaggr_environment" "production_eu" {
  project_id            = flaggr_project.platform.id
  slug                  = "production-eu"
  name                  = "Production (EU)"
  order                 = 90
  protected_environment = true
  require_approval      = true
}
