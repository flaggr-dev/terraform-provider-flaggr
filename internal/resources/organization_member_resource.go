package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/flaggr-dev/terraform-provider-flaggr/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &OrganizationMemberResource{}
var _ resource.ResourceWithImportState = &OrganizationMemberResource{}

type OrganizationMemberResource struct {
	client *client.Client
}

type OrganizationMemberResourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Email          types.String `tfsdk:"email"`
	Role           types.String `tfsdk:"role"`
	UserID         types.String `tfsdk:"user_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

// emailAddDeprecation explains why `email` no longer adds anyone: the API
// never looks an address up and adds the account — it emails an invitation,
// which API tokens and personal access tokens may not create.
const emailAddDeprecation = "Adding an organization member by email is no longer supported: Flaggr emails an " +
	"invitation instead of adding the account, and API tokens can't create invitations. Set user_id (someone who " +
	"already shares an organization or project with you), or invite them from the Flaggr dashboard and import the " +
	"membership once they accept (terraform import <address> orgId:membershipId)."

func NewOrganizationMemberResource() resource.Resource {
	return &OrganizationMemberResource{}
}

func (r *OrganizationMemberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_member"
}

func (r *OrganizationMemberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a member of a Flaggr organization. Members are added by user ID, and only people who " +
			"already share an organization or project with the provider's credential can be added (anyone else: invite " +
			"them by email from the Flaggr dashboard, then import the membership once they accept). Destroying it also " +
			"removes the member's direct memberships and team seats in the organization's projects and withdraws " +
			"pending invitations addressed to them. Destroy fails with 400 when they are the organization's last " +
			"owner, or the last owner of one of its projects and the provider is not authenticated as an organization " +
			"owner (an owner's credential becomes that project's owner first), and with 403 when the credential may not " +
			"make ownership changes or remove a project owner.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Membership ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				Description: "Organization ID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"email": schema.StringAttribute{
				Description: "Email of the member, read back from the API. Setting it no longer adds anyone: " +
					emailAddDeprecation,
				Optional:           true,
				Computed:           true,
				DeprecationMessage: emailAddDeprecation,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"role": schema.StringAttribute{
				Description: "Organization role: owner, admin, member, or billing. Only organization owners can grant, change " +
					"or remove the owner role, and only with a credential allowed to make ownership changes (the dashboard, or a " +
					"personal access token created with owner actions); project API tokens get 403. Admins assign admin, member " +
					"or billing. A role other than billing gives access to every project in the organization, so each project's " +
					"plan needs a free member seat (402 otherwise).",
				Required: true,
			},
			"user_id": schema.StringAttribute{
				Description: "User ID of the member to add. It must be someone who already shares an organization or project " +
					"with the provider's credential; an unknown ID and an unrelated one both get 404.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Creation timestamp.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "Last update timestamp.",
				Computed:    true,
			},
		},
	}
}

func (r *OrganizationMemberResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData))
		return
	}
	r.client = c
}

// knownString reports whether v holds a non-empty, known value.
func knownString(v types.String) bool {
	return !v.IsNull() && !v.IsUnknown() && v.ValueString() != ""
}

// orgMemberCreateInput is the POST body for a direct add, which works by user
// ID only. An email alone can't add anyone: the API answers an address with an
// emailed invitation, never by adding the account.
func orgMemberCreateInput(plan OrganizationMemberResourceModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	if knownString(plan.UserID) {
		return map[string]interface{}{
			"userId": plan.UserID.ValueString(),
			"role":   plan.Role.ValueString(),
		}, diags
	}
	if knownString(plan.Email) {
		diags.AddAttributeError(path.Root("email"), "Adding a member by email is not supported", emailAddDeprecation)
		return nil, diags
	}
	diags.AddAttributeError(path.Root("user_id"), "Missing user_id",
		"Set user_id to the ID of someone who already shares an organization or project with you.")
	return nil, diags
}

// emailFromMember keeps a known email and otherwise takes the one the API lists
// (null when it lists none), so state never holds an unknown value.
func emailFromMember(current types.String, member *client.OrgMember) types.String {
	if !current.IsNull() && !current.IsUnknown() {
		return current
	}
	if member != nil && member.User != nil && member.User.Email != "" {
		return types.StringValue(member.User.Email)
	}
	return types.StringNull()
}

func (r *OrganizationMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan OrganizationMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input, diags := orgMemberCreateInput(plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	member, err := r.client.AddOrgMember(ctx, plan.OrganizationID.ValueString(), input)
	if err != nil {
		resp.Diagnostics.AddError("Error adding org member", err.Error())
		return
	}

	plan.ID = types.StringValue(member.ID)
	plan.UserID = types.StringValue(member.UserID)
	plan.Email = emailFromMember(plan.Email, member)
	plan.CreatedAt = types.StringValue(member.CreatedAt)
	plan.UpdatedAt = types.StringValue(member.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *OrganizationMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state OrganizationMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	members, err := r.client.ListOrgMembers(ctx, state.OrganizationID.ValueString())
	if err != nil {
		if client.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading org members", err.Error())
		return
	}

	var found *client.OrgMember
	for _, m := range members {
		if m.ID == state.ID.ValueString() {
			found = &m
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.Role = types.StringValue(found.Role)
	state.UserID = types.StringValue(found.UserID)
	state.Email = emailFromMember(state.Email, found)
	state.CreatedAt = types.StringValue(found.CreatedAt)
	state.UpdatedAt = types.StringValue(found.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *OrganizationMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan OrganizationMemberResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state OrganizationMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := map[string]interface{}{
		"role": plan.Role.ValueString(),
	}

	member, err := r.client.UpdateOrgMember(ctx, state.OrganizationID.ValueString(), state.ID.ValueString(), input)
	if err != nil {
		resp.Diagnostics.AddError("Error updating org member", err.Error())
		return
	}

	plan.ID = state.ID
	plan.UserID = state.UserID
	plan.Email = emailFromMember(state.Email, member)
	plan.CreatedAt = types.StringValue(member.CreatedAt)
	plan.UpdatedAt = types.StringValue(member.UpdatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *OrganizationMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state OrganizationMemberResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.RemoveOrgMember(ctx, state.OrganizationID.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFoundError(err) {
		resp.Diagnostics.AddError("Error removing org member", err.Error())
	}
}

func (r *OrganizationMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: orgId:memberId
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: orgId:memberId")
		return
	}
	orgID, memberID := parts[0], parts[1]

	members, err := r.client.ListOrgMembers(ctx, orgID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing org member", err.Error())
		return
	}

	var found *client.OrgMember
	for _, m := range members {
		if m.ID == memberID {
			found = &m
			break
		}
	}

	if found == nil {
		resp.Diagnostics.AddError("Member not found", fmt.Sprintf("Member %s not found in organization %s", memberID, orgID))
		return
	}

	state := OrganizationMemberResourceModel{
		ID:             types.StringValue(found.ID),
		OrganizationID: types.StringValue(orgID),
		Role:           types.StringValue(found.Role),
		UserID:         types.StringValue(found.UserID),
		Email:          emailFromMember(types.StringNull(), found),
		CreatedAt:      types.StringValue(found.CreatedAt),
		UpdatedAt:      types.StringValue(found.UpdatedAt),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
