// Package flagkeys holds the flag keys Flaggr refuses for new flags and a
// plan-time validator for them.
package flagkeys

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// Reserved mirrors the Flaggr API's reserved flag keys (RESERVED_FLAG_KEYS in
// the Flaggr application; keep them in sync, TestReservedMatchesTheFlaggrAPI
// checks it when the application source is available). Each names a built-in
// /api/flags/<segment> route, which Next.js
// serves instead of the flag's own /api/flags/{key} route — the URL this
// provider reads, updates and deletes flags through. The API refuses these
// keys for new flags (400 "Flag key is reserved").
var Reserved = []string{
	"bulk",
	"create-from-nl",
	"evaluate",
	"evaluate-debug",
	"export",
	"import",
	"stream",
}

// IsReserved reports whether key is reserved. The match is exact and
// case-sensitive, like Next.js routing (and the API): "Stream" is fine.
func IsReserved(key string) bool {
	for _, reserved := range Reserved {
		if key == reserved {
			return true
		}
	}
	return false
}

// NotReserved refuses reserved flag keys at plan time, so `terraform plan`
// reports them instead of a 400 at apply (or a flag that can't be read).
func NotReserved() validator.String {
	return notReservedValidator{}
}

type notReservedValidator struct{}

func (v notReservedValidator) Description(_ context.Context) string {
	return fmt.Sprintf("must not be a reserved flag key (%s)", strings.Join(Reserved, ", "))
}

func (v notReservedValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v notReservedValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// Unknown values (computed from other resources) are checked once known.
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	key := req.ConfigValue.ValueString()
	if !IsReserved(key) {
		return
	}
	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Reserved flag key",
		fmt.Sprintf(
			"%q is reserved: /api/flags/%s is a built-in Flaggr route, so a flag can't use it as its key "+
				"(Flaggr refuses it, and the flag couldn't be read, updated or deleted through its URL). "+
				"Reserved keys: %s. Choose another key.",
			key, key, strings.Join(Reserved, ", "),
		),
	)
}
