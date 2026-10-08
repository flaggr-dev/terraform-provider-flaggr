package resources

import (
	"strings"
	"testing"

	"github.com/flaggr-dev/terraform-provider-flaggr/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestOrgMemberCreateInputAddsByUserIDOnly(t *testing.T) {
	input, diags := orgMemberCreateInput(OrganizationMemberResourceModel{
		UserID: types.StringValue("u-1"),
		Email:  types.StringValue("ann@acme.com"),
		Role:   types.StringValue("member"),
	})
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}
	if input["userId"] != "u-1" || input["role"] != "member" {
		t.Fatalf("input = %#v", input)
	}
	// An address is never sent to be resolved to an account.
	if _, ok := input["email"]; ok {
		t.Fatalf("email was sent: %#v", input)
	}
}

func TestOrgMemberCreateInputRefusesAnEmailAlone(t *testing.T) {
	_, diags := orgMemberCreateInput(OrganizationMemberResourceModel{
		UserID: types.StringUnknown(),
		Email:  types.StringValue("ann@acme.com"),
		Role:   types.StringValue("member"),
	})
	if !diags.HasError() {
		t.Fatal("an email alone was accepted")
	}
	if detail := diags.Errors()[0].Detail(); !strings.Contains(detail, "invitation") || !strings.Contains(detail, "user_id") {
		t.Fatalf("detail = %q", detail)
	}

	_, diags = orgMemberCreateInput(OrganizationMemberResourceModel{
		UserID: types.StringNull(),
		Email:  types.StringNull(),
		Role:   types.StringValue("member"),
	})
	if !diags.HasError() {
		t.Fatal("neither user_id nor email was accepted")
	}
}

func TestEmailFromMemberKeepsKnownValuesAndNeverLeavesUnknown(t *testing.T) {
	listed := &client.OrgMember{ID: "om-1", User: &client.OrgMemberUser{Email: "Ann@acme.com"}}

	if got := emailFromMember(types.StringValue("ann@acme.com"), listed); got.ValueString() != "ann@acme.com" {
		t.Fatalf("a configured email was replaced: %v", got)
	}
	if got := emailFromMember(types.StringUnknown(), listed); got.ValueString() != "Ann@acme.com" {
		t.Fatalf("unknown email not read from the API: %v", got)
	}
	if got := emailFromMember(types.StringUnknown(), &client.OrgMember{ID: "om-1"}); !got.IsNull() {
		t.Fatalf("email without a listed user = %v, want null", got)
	}
}
