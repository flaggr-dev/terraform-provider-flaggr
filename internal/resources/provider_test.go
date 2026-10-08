package resources_test

import (
	"os"
	"testing"

	tfprovider "github.com/flaggr-dev/terraform-provider-flaggr/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is used to instantiate the provider during
// acceptance tests. The factory function is invoked for every Terraform CLI
// command executed during acceptance testing.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"flaggr": providerserver.NewProtocol6WithError(tfprovider.New("test")()),
}

// testAccPreCheck validates the testing environment before running acceptance tests.
func testAccPreCheck(t *testing.T) {
	if os.Getenv("FLAGGR_API_TOKEN") == "" {
		t.Fatal(`FLAGGR_API_TOKEN must be an admin personal access token (fgp_…) created with "Allow owner actions" for acceptance tests`)
	}
}
