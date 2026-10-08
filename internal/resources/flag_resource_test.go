package resources_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFlagResource_boolean(t *testing.T) {
	slug := fmt.Sprintf("tfacc-flag-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create project + service + boolean flag
			{
				Config: testAccFlagConfig(slug, "dark-mode", "Dark Mode", "boolean", "false", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("flaggr_flag.test", "id"),
					resource.TestCheckResourceAttr("flaggr_flag.test", "key", "dark-mode"),
					resource.TestCheckResourceAttr("flaggr_flag.test", "name", "Dark Mode"),
					resource.TestCheckResourceAttr("flaggr_flag.test", "type", "boolean"),
					resource.TestCheckResourceAttr("flaggr_flag.test", "enabled", "true"),
					resource.TestCheckResourceAttr("flaggr_flag.test", "environment", "production"),
					resource.TestCheckResourceAttrPair("flaggr_flag.test", "project_id", "flaggr_project.test", "id"),
					resource.TestCheckResourceAttrPair("flaggr_flag.test", "service_id", "flaggr_service.test", "id"),
				),
			},
			// Update: disable flag
			{
				Config: testAccFlagConfig(slug, "dark-mode", "Dark Mode Updated", "boolean", "false", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flaggr_flag.test", "name", "Dark Mode Updated"),
					resource.TestCheckResourceAttr("flaggr_flag.test", "enabled", "false"),
				),
			},
			// Import
			{
				ResourceName:      "flaggr_flag.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFlagResource_string(t *testing.T) {
	slug := fmt.Sprintf("tfacc-sflag-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFlagConfig(slug, "banner-text", "Banner Text", "string", `"Welcome!"`, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flaggr_flag.test", "key", "banner-text"),
					resource.TestCheckResourceAttr("flaggr_flag.test", "type", "string"),
					resource.TestCheckResourceAttr("flaggr_flag.test", "enabled", "true"),
				),
			},
		},
	})
}

func testAccFlagConfig(slug, key, name, flagType, defaultValue string, enabled bool) string {
	return fmt.Sprintf(`
resource "flaggr_project" "test" {
  name = "TF Acc Test Project (Flag)"
  slug = %q
}

resource "flaggr_service" "test" {
  project_id = flaggr_project.test.id
  name       = "TF Acc Test Service (Flag)"
}

resource "flaggr_flag" "test" {
  project_id    = flaggr_project.test.id
  service_id    = flaggr_service.test.id
  key           = %q
  name          = %q
  type          = %q
  enabled       = %t
  default_value = jsonencode(%s)
  environment   = "production"
}
`, slug, key, name, flagType, enabled, defaultValue)
}
