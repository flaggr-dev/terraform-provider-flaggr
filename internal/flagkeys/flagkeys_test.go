package flagkeys

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func validate(t *testing.T, value types.String) validator.StringResponse {
	t.Helper()
	var resp validator.StringResponse
	NotReserved().ValidateString(context.Background(), validator.StringRequest{
		Path:        path.Root("key"),
		ConfigValue: value,
	}, &resp)
	return resp
}

func TestNotReservedRefusesEveryReservedKeyAtPlanTime(t *testing.T) {
	for _, key := range Reserved {
		resp := validate(t, types.StringValue(key))
		if !resp.Diagnostics.HasError() {
			t.Fatalf("key %q: no error", key)
		}
		diag := resp.Diagnostics.Errors()[0]
		if diag.Summary() != "Reserved flag key" || !strings.Contains(diag.Detail(), "/api/flags/"+key) {
			t.Fatalf("key %q: diagnostic = %q / %q", key, diag.Summary(), diag.Detail())
		}
	}
}

func TestNotReservedAcceptsOtherKeys(t *testing.T) {
	// Exact and case-sensitive, like the API and Next.js routing.
	for _, key := range []string{"dark-mode", "Stream", "EXPORT", "streams", "bulk-import", "evaluate_v2"} {
		if resp := validate(t, types.StringValue(key)); resp.Diagnostics.HasError() {
			t.Fatalf("key %q refused: %v", key, resp.Diagnostics)
		}
	}
	// Unknown (computed elsewhere) and null values are checked once known.
	if resp := validate(t, types.StringUnknown()); resp.Diagnostics.HasError() {
		t.Fatalf("unknown refused: %v", resp.Diagnostics)
	}
	if resp := validate(t, types.StringNull()); resp.Diagnostics.HasError() {
		t.Fatalf("null refused: %v", resp.Diagnostics)
	}
}

// The list mirrors RESERVED_FLAG_KEYS in the Flaggr app. Skipped where the
// app source isn't next to the provider (a standalone checkout).
func TestReservedMatchesTheFlaggrAPI(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "src", "lib", "validation", "flag-schemas.ts"))
	if err != nil {
		t.Skipf("Flaggr app source not available: %v", err)
	}
	block := regexp.MustCompile(`(?s)export const RESERVED_FLAG_KEYS[^=]*=\s*\[(.*?)\]`).FindSubmatch(source)
	if block == nil {
		t.Fatal("RESERVED_FLAG_KEYS not found in flag-schemas.ts")
	}
	var api []string
	for _, m := range regexp.MustCompile(`"([^"]+)"`).FindAllSubmatch(block[1], -1) {
		api = append(api, string(m[1]))
	}
	ours := append([]string(nil), Reserved...)
	sort.Strings(api)
	sort.Strings(ours)
	if strings.Join(api, ",") != strings.Join(ours, ",") {
		t.Fatalf("provider reserves %v, the API reserves %v", ours, api)
	}
}
