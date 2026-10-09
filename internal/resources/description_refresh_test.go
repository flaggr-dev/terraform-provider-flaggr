package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// stateOf is the state that holds the model.
func stateOf(t *testing.T, r resource.Resource, model interface{}) tfsdk.State {
	t.Helper()
	state := tfsdk.State{Schema: schemaOf(t, r).Schema}
	if diags := state.Set(context.Background(), model); diags.HasError() {
		t.Fatalf("state: %v", diags)
	}
	return state
}

// Flaggr answers a description it doesn't have, and one that was cleared, the
// same way: as "". A refresh has to turn that into no description in the state
// whenever the state has one, or a description removed in Flaggr stays in the
// state for good: the dashboard's removal goes unnoticed (the next plan should
// put the configured description back), and so does the approval of a change
// request that removed it (the plan should have nothing left to do, and it
// planned the removal again, which opened another change request).
//
// A state without a description, or with an empty one, is what a configuration
// without one, or with "", plans over: those stay as they are.
func TestRefreshTakesTheDescriptionFlaggrHas(t *testing.T) {
	null, empty := types.StringNull(), types.StringValue("")
	oldText, newText := types.StringValue("Old text"), types.StringValue("New text")

	for _, tc := range descriptionCases() {
		for _, row := range []struct {
			name   string
			stored string       // what Flaggr has
			prior  types.String // the state before the refresh
			want   types.String // the state after it
		}{
			{"removed in Flaggr", "", oldText, null},
			{"changed in Flaggr", "New text", oldText, newText},
			{"unchanged", "Old text", oldText, oldText},
			{"set in Flaggr over none", "New text", null, newText},
			{"set in Flaggr over an empty one", "New text", empty, newText},
			{"none before and now", "", null, null},
			{"empty before and now", "", empty, empty},
		} {
			t.Run(tc.name+"/"+row.name, func(t *testing.T) {
				api := tc.api(row.stored)
				r := tc.resource(api.client(t))

				got := descriptionIn(t, refreshResource(t, r, stateOf(t, r, tc.model(row.prior))))
				if !got.Equal(row.want) {
					t.Errorf("Flaggr has %q: the state's description went from %s to %s, want %s", row.stored, row.prior, got, row.want)
				}
			})
		}
	}
}

// updatesNothing is a resource whose Update returns without setting a state.
type updatesNothing struct{ resource.Resource }

func (updatesNothing) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}

// The framework starts an Update's response from the prior state ("require
// explicit provider updates for tracking successful updates"), so an Update
// that returns without setting one, which is what a failed update does, leaves
// the state as it was. updateResource has to do the same: starting from the
// plan instead, it showed the planned values as the result of an Update that
// never wrote them.
func TestUpdateResourceStartsFromThePriorState(t *testing.T) {
	for _, tc := range descriptionCases() {
		t.Run(tc.name, func(t *testing.T) {
			r := updatesNothing{tc.resource(nil)}
			planned, prior := tc.model(types.StringValue("New text")), tc.model(types.StringValue("Old text"))

			resp := updateResource(t, r, planned, prior)

			if got := descriptionIn(t, resp.State); !got.Equal(types.StringValue("Old text")) {
				t.Errorf("state after an Update that set none: description = %s, want the prior \"Old text\" (not the planned \"New text\")", got)
			}
		})
	}
}
