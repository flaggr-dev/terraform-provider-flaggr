resource "flaggr_project" "platform" {
  name        = "Platform"
  slug        = "platform"
  description = "Main platform project"

  # Optional: defaults to your only organization. Fixed once the project exists.
  organization_id = flaggr_organization.acme.id
}
