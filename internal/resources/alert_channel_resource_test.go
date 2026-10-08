package resources_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAlertChannelResource_basic(t *testing.T) {
	slug := fmt.Sprintf("tfacc-ach-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create webhook alert channel
			{
				Config: testAccAlertChannelConfig(slug, "TF Acc Webhook Channel", "webhook", `{"url":"https://example.com/webhook"}`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("flaggr_alert_channel.test", "id"),
					resource.TestCheckResourceAttr("flaggr_alert_channel.test", "name", "TF Acc Webhook Channel"),
					resource.TestCheckResourceAttr("flaggr_alert_channel.test", "type", "webhook"),
					resource.TestCheckResourceAttr("flaggr_alert_channel.test", "enabled", "true"),
					resource.TestCheckResourceAttrPair("flaggr_alert_channel.test", "project_id", "flaggr_project.test", "id"),
					resource.TestCheckResourceAttrSet("flaggr_alert_channel.test", "created_at"),
					resource.TestCheckResourceAttrSet("flaggr_alert_channel.test", "updated_at"),
				),
			},
			// Update name
			{
				Config: testAccAlertChannelConfig(slug, "TF Acc Webhook Channel Updated", "webhook", `{"url":"https://example.com/webhook-v2"}`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flaggr_alert_channel.test", "name", "TF Acc Webhook Channel Updated"),
				),
			},
			// Import
			{
				ResourceName:            "flaggr_alert_channel.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config"},
			},
		},
	})
}

func testAccAlertChannelConfig(slug, name, channelType, configJSON string) string {
	return fmt.Sprintf(`
resource "flaggr_project" "test" {
  name = "TF Acc Test Project (AlertChannel)"
  slug = %q
}

resource "flaggr_alert_channel" "test" {
  project_id = flaggr_project.test.id
  name       = %q
  type       = %q
  config     = jsonencode(%s)
}
`, slug, name, channelType, configJSON)
}
