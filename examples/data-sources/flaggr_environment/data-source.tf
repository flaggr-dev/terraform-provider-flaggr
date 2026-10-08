data "flaggr_environment" "production" {
  project_id = data.flaggr_project.platform.id
  slug       = "production"
}

output "production_requires_approval" {
  value = data.flaggr_environment.production.require_approval
}
