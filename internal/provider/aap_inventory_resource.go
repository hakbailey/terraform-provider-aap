package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Ensure the implementation satisfies the expected interfaces
var (
	_ resource.Resource              = &aapInventoryResource{}
	_ resource.ResourceWithConfigure = &aapInventoryResource{}
)

// NewAAPInventoryResource is a helper function to simplify the provider implementation
func NewAAPInventoryResource() resource.Resource {
	return &aapInventoryResource{}
}

// aapInventoryResource is the resource implementation
type aapInventoryResource struct {
	client *AAPClient
}

// Metadata returns the resource type name
func (r *aapInventoryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aap_inventory"
}

// Schema defines the schema for the resource
func (r *aapInventoryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
			},
			"organization": schema.Int64Attribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"description": schema.StringAttribute{
				Optional: true,
			},
			"variables": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
			"groups": schema.SetNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Computed: true,
						},
						"inventory": schema.Int64Attribute{
							Computed: true,
						},
						"name": schema.StringAttribute{
							Required: true,
						},
						"children": schema.SetAttribute{
							Optional:    true,
							ElementType: types.StringType,
						},
						"description": schema.StringAttribute{
							Optional: true,
						},
						"variables": schema.MapAttribute{
							Optional:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
			"hosts": schema.SetNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Computed: true,
						},
						"inventory": schema.Int64Attribute{
							Computed: true,
						},
						"name": schema.StringAttribute{
							Required: true,
						},
						"description": schema.StringAttribute{
							Optional: true,
						},
						"groups": schema.SetAttribute{
							Optional:    true,
							ElementType: types.StringType,
						},
						"variables": schema.MapAttribute{
							Optional:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
		},
	}
}

// Create creates the resource and sets the new Terraform state on success.
func (r *aapInventoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Get current state
	var state aapInventoryResourceModel
	diags := req.Plan.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert state resource to API request model
	inventory := AapInventory{
		Organization: 1, // TODO: Using default organization for now, need to update
		Name:         state.Name.ValueString(),
		Description:  state.Description.ValueString(),
	}

	// Convert inventory variables to API request model
	variables, diags := VariablesMapToString(ctx, state.Variables)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	inventory.Variables = variables

	// Generate API request body
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(inventory)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error generating AAP inventory request body",
			"Could not generate request body to create AAP inventory, unexpected error: "+err.Error(),
		)
		return
	}

	// Create new inventory in AAP
	newInventory, err := r.client.CreateInventory(&buf)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating AAP inventory",
			"Could not create AAP inventory, unexpected error: "+err.Error(),
		)
		return
	}

	// Map response body to resource schema and populate computed attribute values
	state.ID = types.Int64Value(newInventory.Id)
	state.Organization = types.Int64Value(newInventory.Organization)
	state.Name = types.StringValue(newInventory.Name)
	if newInventory.Description != "" {
		state.Description = types.StringValue(newInventory.Description)
	} else {
		state.Description = types.StringNull()
	}

	newVariables, diags := VariablesStringToMap(ctx, newInventory.Variables)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Variables = newVariables

	/////////////////////////////
	// Create inventory groups //
	/////////////////////////////

	// Convert inventory groups set to slice of Objects
	groups := make([]types.Object, 0, len(state.Groups.Elements()))
	diags = state.Groups.ElementsAs(ctx, &groups, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var newGroups []AapGroup

	for _, g := range groups {
		// Read group data into resource model
		var groupResource aapGroupResourceModel
		diags := g.As(ctx, &groupResource, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Convert group resource to API request model
		variables, diags := VariablesMapToString(ctx, groupResource.Variables)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		group := AapGroup{
			Inventory:   state.ID.ValueInt64(),
			Name:        groupResource.Name.ValueString(),
			Description: groupResource.Description.ValueString(),
			Variables:   variables,
		}

		// Generate API request body
		var buf bytes.Buffer
		err = json.NewEncoder(&buf).Encode(group)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error generating AAP group request body",
				"Could not generate request body to create AAP group, unexpected error: "+err.Error(),
			)
			return
		}

		// Create new group in AAP
		newGroup, err := r.client.CreateGroup(&buf)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating AAP group",
				"Could not create AAP group, unexpected error: "+err.Error(),
			)
			return
		}

		// Update group struct values for later reference
		group.Id = newGroup.Id

		children := make([]string, 0, len(groupResource.Children.Elements()))
		diags = groupResource.Children.ElementsAs(ctx, &children, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		group.Children = children

		newGroups = append(newGroups, group)

	}

	// Add all children to parent groups
	for _, group := range newGroups {
		for _, childName := range group.Children {
			childId, err := GroupIdFromName(childName, newGroups)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error retrieving group ID",
					"Could not retrieve ID for child group, unexpected error: "+err.Error(),
				)
				return
			}

			parentId := strconv.Itoa(int(group.Id))
			err = r.client.AddChildToGroup(parentId, childId)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error creating AAP group child",
					"Could not create AAP group child, unexpected error: "+err.Error(),
				)
				return
			}
		}
	}

	// 	Map new groups to schema and update state
	schemaGroups, diags := GroupsToSchema(ctx, newGroups)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	stateGroups, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: groupTypes}, schemaGroups)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Groups = stateGroups

	////////////////////////////
	// Create inventory hosts //
	////////////////////////////

	// Convert inventory host set to slice of Objects
	hosts := make([]types.Object, 0, len(state.Hosts.Elements()))
	diags = state.Hosts.ElementsAs(ctx, &hosts, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var newHosts []AapHost

	for _, h := range hosts {
		// Read host data into resource model
		var hostResource aapHostResourceModel
		diags := h.As(ctx, &hostResource, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Convert host resource to API request model
		variables, diags := VariablesMapToString(ctx, hostResource.Variables)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		host := AapHost{
			Inventory:   state.ID.ValueInt64(),
			Name:        hostResource.Name.ValueString(),
			Description: hostResource.Description.ValueString(),
			Variables:   variables,
		}

		// Generate API request body
		var buf bytes.Buffer
		err = json.NewEncoder(&buf).Encode(host)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error generating AAP host request body",
				"Could not generate request body to create AAP host, unexpected error: "+err.Error(),
			)
			return
		}

		// Create new host in AAP
		newHost, err := r.client.CreateHost(&buf)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating AAP host",
				"Could not create AAP host, unexpected error: "+err.Error(),
			)
			return
		}

		// Update host struct values for later reference
		host.Id = newHost.Id

		hostGroups := make([]string, 0, len(hostResource.Groups.Elements()))
		diags = hostResource.Groups.ElementsAs(ctx, &hostGroups, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		host.Groups = hostGroups

		newHosts = append(newHosts, host)

	}

	// Add all groups to hosts
	for _, host := range newHosts {
		for _, groupName := range host.Groups {
			groupId, err := GroupIdFromName(groupName, newGroups)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error retrieving group ID",
					"Could not retrieve ID for host group, unexpected error: "+err.Error(),
				)
				return
			}

			hostId := strconv.Itoa(int(host.Id))
			err = r.client.AddGroupToHost(hostId, groupId)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error adding AAP group to host",
					"Could not add AAP group to host, unexpected error: "+err.Error(),
				)
				return
			}
		}
	}

	// 	Map new hosts back to schema and update state
	schemaHosts, diags := HostsToSchema(ctx, newHosts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	stateHosts, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: hostTypes}, schemaHosts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Hosts = stateHosts

	// Set state to fully populated inventory data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *aapInventoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state aapInventoryResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get inventory value from AAP
	inventory, err := r.client.GetInventory(state.ID.String())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading AAP inventory",
			"Could not retrieve AAP inventory with ID "+state.ID.String()+": "+err.Error(),
		)
		return
	}

	// Overwrite state with retrieved inventory data
	state.ID = types.Int64Value(inventory.Id)
	state.Organization = types.Int64Value(inventory.Organization)
	state.Name = types.StringValue(inventory.Name)
	if inventory.Description != "" {
		state.Description = types.StringValue(inventory.Description)
	} else {
		state.Description = types.StringNull()
	}

	variables, diags := VariablesStringToMap(ctx, inventory.Variables)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Variables = variables

	//////////////////////////
	// Add inventory groups //
	//////////////////////////

	// Get inventory groups from AAP
	groups, err := r.client.GetInventoryGroups(state.ID.String())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading AAP inventory groups",
			"Could not read AAP groups from inventory with ID "+state.ID.String()+": "+err.Error(),
		)
		return
	}

	// Get groups' children from AAP
	for i, group := range groups {
		var childNames []string
		groupId := strconv.Itoa(int(group.Id))
		groupChildren, err := r.client.GetGroupChildren(groupId)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error reading AAP group children",
				"Could not retrieve children from group with ID "+groupId+": "+err.Error(),
			)
			return
		}
		for _, child := range groupChildren {
			childNames = append(childNames, child.Name)
		}
		groups[i].Children = childNames
	}

	// Map groups to schema and update state
	updatedGroups, diags := GroupsToSchema(ctx, groups)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	stateGroups, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: groupTypes}, updatedGroups)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Groups = stateGroups

	/////////////////////////
	// Add inventory hosts //
	/////////////////////////

	// Get inventory hosts from AAP
	hosts, err := r.client.GetInventoryHosts(state.ID.String())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading AAP inventory hosts",
			"Could not read AAP hosts from inventory with ID "+state.ID.String()+": "+err.Error(),
		)
		return
	}

	// Get hosts' groups from AAP
	for i, host := range hosts {
		var groupNames []string
		hostId := strconv.Itoa(int(host.Id))
		hostGroups, err := r.client.GetHostGroups(hostId)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error reading AAP host groups",
				"Could not retrieve groups for host with ID "+hostId+": "+err.Error(),
			)
			return
		}
		for _, group := range hostGroups {
			groupNames = append(groupNames, group.Name)
		}
		hosts[i].Groups = groupNames
	}

	// Map hosts to schema and update state
	updatedHosts, diags := HostsToSchema(ctx, hosts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	stateHosts, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: hostTypes}, updatedHosts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Hosts = stateHosts

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *aapInventoryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *aapInventoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

// Configure adds the provider configured client to the resource.
func (r *aapInventoryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*AAPClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *AAPClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// Given a group ID, retrieves its name from a slice of AapGroup structs
func GroupIdFromName(name string, groups []AapGroup) (int64, error) {
	for _, group := range groups {
		if group.Name == name {
			return group.Id, nil
		}
	}

	err := fmt.Errorf("unable to retrieve ID for group named '%q'", name)
	return 0, err
}

// Convert groups from Go types to schema
func GroupsToSchema(ctx context.Context, groups []AapGroup) ([]types.Object, diag.Diagnostics) {
	var schemaGroups []types.Object
	var resultDiags diag.Diagnostics

	for _, group := range groups {
		groupValues := map[string]attr.Value{
			"id":          types.Int64Value(group.Id),
			"inventory":   types.Int64Value(group.Inventory),
			"name":        types.StringValue(group.Name),
			"children":    types.SetNull(types.StringType),
			"description": types.StringNull(),
			"variables":   types.MapNull(types.StringType),
		}

		children, diags := types.SetValueFrom(ctx, types.StringType, group.Children)
		resultDiags.Append(diags...)
		if diags.HasError() {
			return schemaGroups, diags
		}
		if len(children.Elements()) > 0 {
			groupValues["children"] = children
		}

		if group.Description != "" {
			groupValues["description"] = types.StringValue(group.Description)
		}

		variables, diags := VariablesStringToMap(ctx, group.Variables)
		resultDiags.Append(diags...)
		if resultDiags.HasError() {
			return schemaGroups, diags
		}
		if len(variables.Elements()) > 0 {
			groupValues["variables"] = variables
		}

		groupValue, diags := types.ObjectValue(groupTypes, groupValues)
		resultDiags.Append(diags...)
		if resultDiags.HasError() {
			return schemaGroups, diags
		}
		schemaGroups = append(schemaGroups, groupValue)
	}
	return schemaGroups, resultDiags
}

func HostsToSchema(ctx context.Context, hosts []AapHost) ([]types.Object, diag.Diagnostics) {
	var schemaHosts []types.Object
	var resultDiags diag.Diagnostics

	for _, host := range hosts {
		hostValues := map[string]attr.Value{
			"id":          types.Int64Value(host.Id),
			"inventory":   types.Int64Value(host.Inventory),
			"name":        types.StringValue(host.Name),
			"groups":      types.SetNull(types.StringType),
			"description": types.StringNull(),
			"variables":   types.MapNull(types.StringType),
		}

		if host.Description != "" {
			hostValues["description"] = types.StringValue(host.Description)
		}

		hostGroups, diags := types.SetValueFrom(ctx, types.StringType, host.Groups)
		resultDiags.Append(diags...)
		if diags.HasError() {
			return schemaHosts, diags
		}
		if len(hostGroups.Elements()) > 0 {
			hostValues["groups"] = hostGroups
		}

		variables, diags := VariablesStringToMap(ctx, host.Variables)
		resultDiags.Append(diags...)
		if resultDiags.HasError() {
			return schemaHosts, diags
		}
		if len(variables.Elements()) > 0 {
			hostValues["variables"] = variables
		}

		hostValue, diags := types.ObjectValue(hostTypes, hostValues)
		resultDiags.Append(diags...)
		if resultDiags.HasError() {
			return schemaHosts, diags
		}
		schemaHosts = append(schemaHosts, hostValue)
	}
	return schemaHosts, resultDiags
}

// Converts variables from TF framework MapType to a JSON encoded string
func VariablesMapToString(ctx context.Context, resourceVariables basetypes.MapValue) (string, diag.Diagnostics) {
	variables := make(map[string]string, len(resourceVariables.Elements()))
	diagnostics := resourceVariables.ElementsAs(ctx, &variables, false)

	variablesJson, err := json.Marshal(variables)
	if err != nil {
		diagnostics.AddError(
			"Error marshalling variables map",
			"Could not convert variables map to string, unexpected error: "+err.Error(),
		)
		return "", diagnostics
	}

	return string(variablesJson), diagnostics
}

// Converts variables from a JSON encoded string to a TF framework MapType
func VariablesStringToMap(ctx context.Context, variables string) (basetypes.MapValue, diag.Diagnostics) {

	var diagnostics diag.Diagnostics
	var newVariables map[string]string
	err := json.Unmarshal([]byte(variables), &newVariables)
	if err != nil {
		diagnostics.AddError(
			"Error unmarshalling variables string",
			"Could not convert variables string to map, unexpected error: "+err.Error(),
		)
		return types.MapNull(types.StringType), diagnostics
	}

	mapVariables := types.MapNull(types.StringType)
	var diags diag.Diagnostics
	if len(newVariables) > 0 {
		mapVariables, diags = types.MapValueFrom(ctx, types.StringType, newVariables)
		diagnostics.Append(diags...)
	}
	return mapVariables, diagnostics
}

// aapInventoryResourceModel maps the inventory resource schema data
type aapInventoryResourceModel struct {
	ID           types.Int64  `tfsdk:"id"`
	Organization types.Int64  `tfsdk:"organization"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Variables    types.Map    `tfsdk:"variables"`
	Groups       types.Set    `tfsdk:"groups"`
	Hosts        types.Set    `tfsdk:"hosts"`
}

// aapGroupResourceModel maps AAP the inventory resource's group schema data
type aapGroupResourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Inventory   types.Int64  `tfsdk:"inventory"`
	Name        types.String `tfsdk:"name"`
	Children    types.Set    `tfsdk:"children"`
	Description types.String `tfsdk:"description"`
	Variables   types.Map    `tfsdk:"variables"`
}

// aapHostResourceModel maps AAP the inventory resource's host schema data
type aapHostResourceModel struct {
	Id          types.Int64  `tfsdk:"id"`
	Inventory   types.Int64  `tfsdk:"inventory"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Groups      types.Set    `tfsdk:"groups"`
	Variables   types.Map    `tfsdk:"variables"`
}

// TF framework types for group conversion from Go types
var groupTypes = map[string]attr.Type{
	"id":          types.Int64Type,
	"inventory":   types.Int64Type,
	"name":        types.StringType,
	"children":    basetypes.SetType{ElemType: types.StringType},
	"description": types.StringType,
	"variables":   basetypes.MapType{ElemType: types.StringType},
}

// TF framework types for host conversion from Go types
var hostTypes = map[string]attr.Type{
	"id":          types.Int64Type,
	"inventory":   types.Int64Type,
	"name":        types.StringType,
	"groups":      types.SetType{ElemType: types.StringType},
	"description": types.StringType,
	"variables":   basetypes.MapType{ElemType: types.StringType},
}
