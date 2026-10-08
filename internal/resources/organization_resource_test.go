package resources_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOrganizationResource_basic(t *testing.T) {
	slug := fmt.Sprintf("tfacc-org-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create organization
			{
				Config: testAccOrganizationConfig(slug, "TF Acc Test Org", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("flaggr_organization.test", "id"),
					resource.TestCheckResourceAttr("flaggr_organization.test", "name", "TF Acc Test Org"),
					resource.TestCheckResourceAttr("flaggr_organization.test", "slug", slug),
					resource.TestCheckResourceAttrSet("flaggr_organization.test", "created_at"),
					resource.TestCheckResourceAttrSet("flaggr_organization.test", "updated_at"),
				),
			},
			// Update description
			{
				Config: testAccOrganizationConfig(slug, "TF Acc Test Org Updated", "Updated org description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flaggr_organization.test", "name", "TF Acc Test Org Updated"),
					resource.TestCheckResourceAttr("flaggr_organization.test", "description", "Updated org description"),
				),
			},
			// Import
			{
				ResourceName:      "flaggr_organization.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccOrganizationConfig(slug, name, description string) string {
	descAttr := ""
	if description != "" {
		descAttr = fmt.Sprintf(`  description = %q`, description)
	}
	return fmt.Sprintf(`
resource "flaggr_organization" "test" {
  name = %q
  slug = %q
%s
}
`, name, slug, descAttr)
}
