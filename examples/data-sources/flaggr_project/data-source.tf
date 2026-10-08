data "flaggr_project" "platform" {
  slug = "platform"
}

output "project_id" {
  value = data.flaggr_project.platform.id
}
