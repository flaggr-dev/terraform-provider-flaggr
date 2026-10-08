data "flaggr_organization" "acme" {
  slug = "acme"
}

output "organization_id" {
  value = data.flaggr_organization.acme.id
}
