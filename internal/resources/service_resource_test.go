package resources_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccServiceResource_basic(t *testing.T) {
	slug := fmt.Sprintf("tfacc-svc-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create project + service
			{
				Config: testAccServiceConfig(slug, "TF Acc Test Service", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("flaggr_service.test", "id"),
					resource.TestCheckResourceAttr("flaggr_service.test", "name", "TF Acc Test Service"),
					resource.TestCheckResourceAttrSet("flaggr_service.test", "slug"),
					resource.TestCheckResourceAttrPair("flaggr_service.test", "project_id", "flaggr_project.test", "id"),
					resource.TestCheckResourceAttrSet("flaggr_service.test", "created_at"),
				),
			},
			// Update name
			{
				Config: testAccServiceConfig(slug, "TF Acc Test Service Updated", "Updated service"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flaggr_service.test", "name", "TF Acc Test Service Updated"),
					resource.TestCheckResourceAttr("flaggr_service.test", "description", "Updated service"),
				),
			},
			// Import
			{
				ResourceName:      "flaggr_service.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccServiceConfig(slug, name, description string) string {
	descAttr := ""
	if description != "" {
		descAttr = fmt.Sprintf(`  description = %q`, description)
	}
	return fmt.Sprintf(`
resource "flaggr_project" "test" {
  name = "TF Acc Test Project (Service)"
  slug = %q
}

resource "flaggr_service" "test" {
  project_id = flaggr_project.test.id
  name       = %q
%s
}
`, slug, name, descAttr)
}
