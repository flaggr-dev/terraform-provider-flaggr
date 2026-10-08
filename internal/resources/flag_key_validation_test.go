package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// flaggr_flag.key refuses the keys the API reserves (built-in /api/flags
// routes) at plan time, instead of a 400 at apply.
func TestFlagKeyRefusesReservedKeysAtPlanTime(t *testing.T) {
	s := schemaOf(t, &FlagResource{}).Schema
	attr, ok := s.Attributes["key"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("key attribute = %T", s.Attributes["key"])
	}
	if len(attr.Validators) == 0 {
		t.Fatal("flaggr_flag.key has no validators")
	}

	run := func(key string) bool {
		refused := false
		for _, v := range attr.Validators {
			var resp validator.StringResponse
			v.ValidateString(context.Background(), validator.StringRequest{Path: path.Root("key"), ConfigValue: types.StringValue(key)}, &resp)
			refused = refused || resp.Diagnostics.HasError()
		}
		return refused
	}
	for _, key := range []string{"stream", "evaluate", "evaluate-debug", "export", "import", "bulk", "create-from-nl"} {
		if !run(key) {
			t.Errorf("reserved key %q was accepted", key)
		}
	}
	for _, key := range []string{"dark-mode", "Stream", "new-checkout"} {
		if run(key) {
			t.Errorf("key %q was refused", key)
		}
	}
}
