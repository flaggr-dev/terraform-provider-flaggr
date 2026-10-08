package resources_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProjectResource_basic(t *testing.T) {
	slug := fmt.Sprintf("tfacc-proj-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and verify
			{
				Config: testAccProjectConfig(slug, "TF Acc Test Project", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("flaggr_project.test", "id"),
					resource.TestCheckResourceAttr("flaggr_project.test", "name", "TF Acc Test Project"),
					resource.TestCheckResourceAttr("flaggr_project.test", "slug", slug),
					resource.TestCheckResourceAttrSet("flaggr_project.test", "created_at"),
					resource.TestCheckResourceAttrSet("flaggr_project.test", "updated_at"),
				),
			},
			// Update name
			{
				Config: testAccProjectConfig(slug, "TF Acc Test Project Updated", "Updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flaggr_project.test", "name", "TF Acc Test Project Updated"),
					resource.TestCheckResourceAttr("flaggr_project.test", "description", "Updated description"),
				),
			},
			// Import
			{
				ResourceName:      "flaggr_project.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccProjectConfig(slug, name, description string) string {
	descAttr := ""
	if description != "" {
		descAttr = fmt.Sprintf(`  description = %q`, description)
	}
	return fmt.Sprintf(`
resource "flaggr_project" "test" {
  name = %q
  slug = %q
%s
}
`, name, slug, descAttr)
}
