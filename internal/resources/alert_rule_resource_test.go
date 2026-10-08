package resources_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAlertRuleResource_basic(t *testing.T) {
	slug := fmt.Sprintf("tfacc-arl-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create alert rule with channel dependency
			{
				Config: testAccAlertRuleConfig(slug, "TF Acc Error Rate Rule", "warning", "error_rate", 5.0, 5, 15),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("flaggr_alert_rule.test", "id"),
					resource.TestCheckResourceAttr("flaggr_alert_rule.test", "name", "TF Acc Error Rate Rule"),
					resource.TestCheckResourceAttr("flaggr_alert_rule.test", "severity", "warning"),
					resource.TestCheckResourceAttr("flaggr_alert_rule.test", "condition_type", "error_rate"),
					resource.TestCheckResourceAttr("flaggr_alert_rule.test", "threshold", "5"),
					resource.TestCheckResourceAttr("flaggr_alert_rule.test", "window_minutes", "5"),
					resource.TestCheckResourceAttr("flaggr_alert_rule.test", "enabled", "true"),
					resource.TestCheckResourceAttr("flaggr_alert_rule.test", "cooldown_minutes", "15"),
					resource.TestCheckResourceAttrPair("flaggr_alert_rule.test", "project_id", "flaggr_project.test", "id"),
					resource.TestCheckResourceAttrSet("flaggr_alert_rule.test", "created_at"),
					resource.TestCheckResourceAttrSet("flaggr_alert_rule.test", "updated_at"),
				),
			},
			// Update threshold and severity
			{
				Config: testAccAlertRuleConfig(slug, "TF Acc Error Rate Rule Updated", "critical", "error_rate", 10.0, 10, 30),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flaggr_alert_rule.test", "name", "TF Acc Error Rate Rule Updated"),
					resource.TestCheckResourceAttr("flaggr_alert_rule.test", "severity", "critical"),
					resource.TestCheckResourceAttr("flaggr_alert_rule.test", "threshold", "10"),
					resource.TestCheckResourceAttr("flaggr_alert_rule.test", "window_minutes", "10"),
					resource.TestCheckResourceAttr("flaggr_alert_rule.test", "cooldown_minutes", "30"),
				),
			},
			// Import
			{
				ResourceName:            "flaggr_alert_rule.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"channels"},
			},
		},
	})
}

func testAccAlertRuleConfig(slug, name, severity, conditionType string, threshold float64, windowMinutes, cooldownMinutes int) string {
	return fmt.Sprintf(`
resource "flaggr_project" "test" {
  name = "TF Acc Test Project (AlertRule)"
  slug = %q
}

resource "flaggr_alert_channel" "test" {
  project_id = flaggr_project.test.id
  name       = "TF Acc Rule Channel"
  type       = "webhook"
  config     = jsonencode({"url": "https://example.com/webhook"})
}

resource "flaggr_alert_rule" "test" {
  project_id      = flaggr_project.test.id
  name            = %q
  severity        = %q
  condition_type  = %q
  threshold       = %g
  window_minutes  = %d
  cooldown_minutes = %d
  channels        = jsonencode([{"channelId": flaggr_alert_channel.test.id, "channelName": flaggr_alert_channel.test.name}])
}
`, slug, name, severity, conditionType, threshold, windowMinutes, cooldownMinutes)
}
