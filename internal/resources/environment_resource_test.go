package resources_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEnvironmentResource_basic(t *testing.T) {
	slug := fmt.Sprintf("tfacc-env-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create environment
			{
				Config: testAccEnvironmentConfig(slug, "qa", "QA", "", "#10b981", 30),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("flaggr_environment.test", "id"),
					resource.TestCheckResourceAttr("flaggr_environment.test", "slug", "qa"),
					resource.TestCheckResourceAttr("flaggr_environment.test", "name", "QA"),
					resource.TestCheckResourceAttr("flaggr_environment.test", "color", "#10b981"),
					resource.TestCheckResourceAttr("flaggr_environment.test", "order", "30"),
					resource.TestCheckResourceAttr("flaggr_environment.test", "protected_environment", "false"),
					resource.TestCheckResourceAttr("flaggr_environment.test", "require_approval", "false"),
					resource.TestCheckResourceAttrPair("flaggr_environment.test", "project_id", "flaggr_project.test", "id"),
					resource.TestCheckResourceAttrSet("flaggr_environment.test", "created_at"),
					resource.TestCheckResourceAttrSet("flaggr_environment.test", "updated_at"),
				),
			},
			// Update description and color
			{
				Config: testAccEnvironmentConfig(slug, "qa", "QA", "Quality assurance environment", "#3b82f6", 30),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flaggr_environment.test", "name", "QA"),
					resource.TestCheckResourceAttr("flaggr_environment.test", "description", "Quality assurance environment"),
					resource.TestCheckResourceAttr("flaggr_environment.test", "color", "#3b82f6"),
				),
			},
			// Import
			{
				ResourceName:            "flaggr_environment.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"is_default"},
			},
		},
	})
}

func testAccEnvironmentConfig(projectSlug, envSlug, name, description, color string, order int) string {
	descAttr := ""
	if description != "" {
		descAttr = fmt.Sprintf(`  description = %q`, description)
	}
	colorAttr := ""
	if color != "" {
		colorAttr = fmt.Sprintf(`  color = %q`, color)
	}
	return fmt.Sprintf(`
resource "flaggr_project" "test" {
  name = "TF Acc Test Project (Env)"
  slug = %q
}

resource "flaggr_environment" "test" {
  project_id = flaggr_project.test.id
  slug       = %q
  name       = %q
%s
%s
  order      = %d
}
`, projectSlug, envSlug, name, descAttr, colorAttr, order)
}
