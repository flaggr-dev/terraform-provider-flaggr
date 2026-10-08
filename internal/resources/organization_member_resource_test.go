package resources_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestAccOrganizationMemberResource_basic adds a member by user ID. Members can
// only be added directly when they already share an organization or project
// with the token's user, so the test needs such a user:
// FLAGGR_TEST_MEMBER_USER_ID (e.g. a colleague in one of your projects).
func TestAccOrganizationMemberResource_basic(t *testing.T) {
	userID := os.Getenv("FLAGGR_TEST_MEMBER_USER_ID")
	if userID == "" && os.Getenv("TF_ACC") != "" {
		t.Skip("FLAGGR_TEST_MEMBER_USER_ID must name a user who shares an organization or project with the token's user")
	}
	slug := fmt.Sprintf("tfacc-orgm-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create org member with "member" role
			{
				Config: testAccOrganizationMemberConfig(slug, userID, "member"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("flaggr_organization_member.test", "id"),
					resource.TestCheckResourceAttr("flaggr_organization_member.test", "user_id", userID),
					resource.TestCheckResourceAttrSet("flaggr_organization_member.test", "email"),
					resource.TestCheckResourceAttr("flaggr_organization_member.test", "role", "member"),
					resource.TestCheckResourceAttrPair("flaggr_organization_member.test", "organization_id", "flaggr_organization.test", "id"),
					resource.TestCheckResourceAttrSet("flaggr_organization_member.test", "created_at"),
					resource.TestCheckResourceAttrSet("flaggr_organization_member.test", "updated_at"),
				),
			},
			// Update role to "admin"
			{
				Config: testAccOrganizationMemberConfig(slug, userID, "admin"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flaggr_organization_member.test", "role", "admin"),
				),
			},
			// Import (email is read back from the member list)
			{
				ResourceName:      "flaggr_organization_member.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Import IDs are orgId:membershipId.
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["flaggr_organization_member.test"]
					if !ok {
						return "", fmt.Errorf("flaggr_organization_member.test not in state")
					}
					return rs.Primary.Attributes["organization_id"] + ":" + rs.Primary.ID, nil
				},
			},
		},
	})
}

func testAccOrganizationMemberConfig(slug, userID, role string) string {
	return fmt.Sprintf(`
resource "flaggr_organization" "test" {
  name = "TF Acc Test Org (Member)"
  slug = %q
}

resource "flaggr_organization_member" "test" {
  organization_id = flaggr_organization.test.id
  user_id         = %q
  role            = %q
}
`, slug, userID, role)
}
