data "flaggr_service" "api" {
  id = "your-service-id"
}

output "service_name" {
  value = data.flaggr_service.api.name
}
