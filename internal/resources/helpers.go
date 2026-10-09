package resources

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func jsonUnmarshalString(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// setUpdateDescription adds the description to the body of an update request
// when the request has to carry one: the planned description, or "" when the
// configuration no longer has one but the prior state does.
//
// The API keeps a field an update doesn't name, and it refuses (or ignores)
// null, so "" is how a description is cleared. Leaving it out instead keeps
// the old text in Flaggr: the next refresh reads it back and every plan shows
// the removal again. Nothing is added when there was no description to clear
// (a prior null or ""), so an update that doesn't touch the description sends
// the same body as before, and a change request in an environment that needs
// approval doesn't list a description change.
func setUpdateDescription(input map[string]interface{}, plan, state types.String) {
	switch {
	case plan.IsUnknown():
		// An apply runs with the final plan, where every configured value is
		// known, so this isn't reached; send nothing rather than guess.
	case !plan.IsNull():
		input["description"] = plan.ValueString()
	case !state.IsNull() && !state.IsUnknown() && state.ValueString() != "":
		input["description"] = ""
	}
}
