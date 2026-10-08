package resources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestProjectUpdateInputSendsSlugOnlyWhenItChanges(t *testing.T) {
	state := ProjectResourceModel{
		Name:        types.StringValue("Old name"),
		Slug:        types.StringValue("my-app"),
		Description: types.StringNull(),
	}

	unchanged := projectUpdateInput(ProjectResourceModel{
		Name:        types.StringValue("New name"),
		Slug:        types.StringValue("my-app"),
		Description: types.StringValue("Production flags"),
	}, state)
	if _, ok := unchanged["slug"]; ok {
		t.Fatalf("unchanged slug was sent: %#v", unchanged)
	}
	if unchanged["name"] != "New name" || unchanged["description"] != "Production flags" {
		t.Fatalf("input = %#v", unchanged)
	}

	renamed := projectUpdateInput(ProjectResourceModel{
		Name:        types.StringValue("Old name"),
		Slug:        types.StringValue("my-app-v2"),
		Description: types.StringNull(),
	}, state)
	if renamed["slug"] != "my-app-v2" {
		t.Fatalf("changed slug was not sent: %#v", renamed)
	}
	if _, ok := renamed["description"]; ok {
		t.Fatalf("null description was sent: %#v", renamed)
	}
}

// CR-5: the API keeps a field it isn't sent and refuses null, so a
// description removed from the configuration is cleared with "".
func TestProjectUpdateInputClearsARemovedDescription(t *testing.T) {
	cleared := projectUpdateInput(ProjectResourceModel{
		Name:        types.StringValue("My app"),
		Slug:        types.StringValue("my-app"),
		Description: types.StringNull(),
	}, ProjectResourceModel{
		Name:        types.StringValue("My app"),
		Slug:        types.StringValue("my-app"),
		Description: types.StringValue("Production flags"),
	})
	if len(cleared) != 1 || cleared["description"] != "" {
		t.Fatalf("input = %#v, want only an empty description", cleared)
	}
}
