resource "flaggr_service" "api" {
  project_id  = flaggr_project.platform.id
  name        = "API Gateway"
  description = "Primary API gateway service"
}
