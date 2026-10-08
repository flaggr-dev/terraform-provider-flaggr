# The user must already share an organization or project with the
# provider's credential. To add someone new, invite them by email from the
# Flaggr dashboard, then import the membership once they accept.
resource "flaggr_organization_member" "alice" {
  organization_id = flaggr_organization.acme.id
  user_id         = "alice-user-id"
  role            = "admin"
}
