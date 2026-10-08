package resources_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMetricSourceResource_basic(t *testing.T) {
	slug := fmt.Sprintf("tfacc-msrc-%d", os.Getpid())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create prometheus metric source
			{
				Config: testAccMetricSourceConfig(slug, "TF Acc Prometheus Source", "prometheus",
					`{"endpoint":"https://prometheus.example.com","queryTemplate":"rate(http_requests_total[5m])"}`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("flaggr_metric_source.test", "id"),
					resource.TestCheckResourceAttr("flaggr_metric_source.test", "name", "TF Acc Prometheus Source"),
					resource.TestCheckResourceAttr("flaggr_metric_source.test", "type", "prometheus"),
					resource.TestCheckResourceAttr("flaggr_metric_source.test", "enabled", "true"),
					resource.TestCheckResourceAttrPair("flaggr_metric_source.test", "project_id", "flaggr_project.test", "id"),
					resource.TestCheckResourceAttrSet("flaggr_metric_source.test", "created_at"),
					resource.TestCheckResourceAttrSet("flaggr_metric_source.test", "updated_at"),
				),
			},
			// Update name
			{
				Config: testAccMetricSourceConfig(slug, "TF Acc Prometheus Source Updated", "prometheus",
					`{"endpoint":"https://prometheus.example.com","queryTemplate":"rate(http_requests_total[10m])"}`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("flaggr_metric_source.test", "name", "TF Acc Prometheus Source Updated"),
				),
			},
			// Import (ignore config — it's Sensitive)
			{
				ResourceName:            "flaggr_metric_source.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config"},
			},
		},
	})
}

func testAccMetricSourceConfig(slug, name, sourceType, configJSON string) string {
	return fmt.Sprintf(`
resource "flaggr_project" "test" {
  name = "TF Acc Test Project (MetricSource)"
  slug = %q
}

resource "flaggr_metric_source" "test" {
  project_id = flaggr_project.test.id
  name       = %q
  type       = %q
  config     = jsonencode(%s)
}
`, slug, name, sourceType, configJSON)
}
