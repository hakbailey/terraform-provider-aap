package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
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
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"organization": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
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
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
						"inventory": schema.Int64Attribute{
							Computed: true,
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
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
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
						"inventory": schema.Int64Attribute{
							Computed: true,
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
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
	// Retrieve values from plan
	var plan aapInventoryResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert state resource to API request model
	inventory := AapInventory{
		Organization: 1, // TODO: Using default organization for now, need to update
		Name:         plan.Name.ValueString(),
		Description:  plan.Description.ValueString(),
	}

	// Convert inventory variables to API request model
	variables, diags := VariablesMapToString(ctx, plan.Variables)
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
	plan.ID = types.Int64Value(newInventory.Id)
	plan.Organization = types.Int64Value(newInventory.Organization)
	plan.Name = types.StringValue(newInventory.Name)
	if newInventory.Description != "" {
		plan.Description = types.StringValue(newInventory.Description)
	} else {
		plan.Description = types.StringNull()
	}

	newVariables, diags := VariablesStringToMap(ctx, newInventory.Variables)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Variables = newVariables

	/////////////////////////////
	// Create inventory groups //
	/////////////////////////////

	// Convert inventory groups set to slice of Objects
	groups := make([]types.Object, 0, len(plan.Groups.Elements()))
	diags = plan.Groups.ElementsAs(ctx, &groups, false)
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
			Inventory:   plan.ID.ValueInt64(),
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
	plan.Groups = stateGroups

	////////////////////////////
	// Create inventory hosts //
	////////////////////////////

	// Convert inventory host set to slice of Objects
	hosts := make([]types.Object, 0, len(plan.Hosts.Elements()))
	diags = plan.Hosts.ElementsAs(ctx, &hosts, false)
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
			Inventory:   plan.ID.ValueInt64(),
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
	plan.Hosts = stateHosts

	// Set state to fully populated inventory data
	diags = resp.State.Set(ctx, plan)
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
	// Retrieve values from plan
	var plan aapInventoryResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert plan resource to API request model
	inventory := AapInventory{
		Organization: plan.Organization.ValueInt64(),
		Name:         plan.Name.ValueString(),
		Description:  plan.Description.ValueString(),
	}

	// Convert inventory variables to API request model
	variables, diags := VariablesMapToString(ctx, plan.Variables)
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
			"Could not generate request body to update AAP inventory, unexpected error: "+err.Error(),
		)
		return
	}

	// Update inventory in AAP
	inventoryId := strconv.Itoa(int(plan.ID.ValueInt64()))
	updatedInventory, err := r.client.UpdateInventory(inventoryId, &buf)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating AAP inventory",
			"Could not update AAP inventory, unexpected error: "+err.Error(),
		)
		return
	}

	// Map response body to resource schema and populate computed attribute values
	plan.ID = types.Int64Value(updatedInventory.Id)
	plan.Organization = types.Int64Value(updatedInventory.Organization)
	plan.Name = types.StringValue(updatedInventory.Name)
	if updatedInventory.Description != "" {
		plan.Description = types.StringValue(updatedInventory.Description)
	} else {
		plan.Description = types.StringNull()
	}

	updatedVariables, diags := VariablesStringToMap(ctx, updatedInventory.Variables)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Variables = updatedVariables

	/////////////////////////////
	// Update inventory groups //
	/////////////////////////////

	// Get inventory's current groups
	currentGroups, err := r.client.GetInventoryGroups(inventoryId)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error retrieving groups for inventory",
			"Could not retrieve groups for inventory "+inventory.Name+", unexpected error: "+err.Error(),
		)
		return
	}

	// Convert plan inventory groups set to slice of Objects
	groups := make([]types.Object, 0, len(plan.Groups.Elements()))
	diags = plan.Groups.ElementsAs(ctx, &groups, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var updatedGroups []AapGroup

	// Create or update groups in plan
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
			Inventory:   plan.ID.ValueInt64(),
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
				"Could not generate request body to update AAP group, unexpected error: "+err.Error(),
			)
			return
		}

		var updatedGroup *AapGroup
		groupId := groupResource.ID.ValueInt64()

		// If group is not in current inventory groups create it, otherwise update it
		groupIndex := slices.IndexFunc(currentGroups, func(g AapGroup) bool { return g.Id == groupId })
		if groupIndex == -1 {
			updatedGroup, err = r.client.CreateGroup(&buf)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error creating AAP group",
					"Could not create AAP group "+group.Name+", unexpected error: "+err.Error(),
				)
				return
			}
		} else {
			updatedGroup, err = r.client.UpdateGroup(strconv.Itoa(int(groupId)), &buf)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error updating AAP group",
					"Could not update AAP group "+group.Name+", unexpected error: "+err.Error(),
				)
				return
			}
		}

		// Update group struct values for later reference
		group.Id = updatedGroup.Id

		children := make([]string, 0, len(groupResource.Children.Elements()))
		diags = groupResource.Children.ElementsAs(ctx, &children, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		group.Children = children

		updatedGroups = append(updatedGroups, group)
	}

	// If any current inventory groups are not in updated plan groups, delete them
	for _, currentGroup := range currentGroups {
		groupIndex := slices.IndexFunc(updatedGroups, func(g AapGroup) bool { return g.Id == currentGroup.Id })
		if groupIndex == -1 {
			groupId := strconv.Itoa(int(currentGroup.Id))
			err = r.client.DeleteGroup(groupId)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error deleting group",
					"Could not delete group "+currentGroup.Name+", unexpected error: "+err.Error(),
				)
				return
			}
		}
	}

	// Ensure all parent groups have updated children
	for _, group := range updatedGroups {
		groupId := strconv.Itoa(int(group.Id))

		// Get group's current children
		currentChildren, err := r.client.GetGroupChildren(groupId)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error retrieving current children for group",
				"Could not retrieve current children for group "+group.Name+", unexpected error: "+err.Error(),
			)
			return
		}

		// If any updated children are not in current children, add them to group
		for _, childName := range group.Children {
			childIndex := slices.IndexFunc(currentChildren, func(g AapGroup) bool { return g.Name == childName })

			if childIndex == -1 {
				childId, err := GroupIdFromName(childName, updatedGroups)
				if err != nil {
					resp.Diagnostics.AddError(
						"Error retrieving group ID",
						"Could not retrieve ID for child group "+childName+", unexpected error: "+err.Error(),
					)
					return
				}

				err = r.client.AddChildToGroup(groupId, childId)
				if err != nil {
					resp.Diagnostics.AddError(
						"Error adding child to group",
						"Could not add child "+childName+" to group "+group.Name+", unexpected error: "+err.Error(),
					)
					return
				}
			}
		}

		// If any current children are not in updated children, remove them from group
		for _, child := range currentChildren {
			containsChild := slices.Contains(group.Children, child.Name)
			if !containsChild {
				err = r.client.RemoveChildFromGroup(groupId, child.Id)
				if err != nil {
					resp.Diagnostics.AddError(
						"Error removing child from group",
						"Could not remove child "+child.Name+" from group "+group.Name+", unexpected error: "+err.Error(),
					)
					return
				}
			}
		}
	}

	// 	Map updated groups to schema and update state
	schemaGroups, diags := GroupsToSchema(ctx, updatedGroups)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updatedStateGroups, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: groupTypes}, schemaGroups)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Groups = updatedStateGroups

	////////////////////////////
	// Update inventory hosts //
	////////////////////////////

	// Get inventory's current hosts
	currentHosts, err := r.client.GetInventoryHosts(inventoryId)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error retrieving hosts for inventory",
			"Could not retrieve hosts for inventory "+inventory.Name+", unexpected error: "+err.Error(),
		)
		return
	}

	// Convert plan inventory hosts set to slice of Objects
	hosts := make([]types.Object, 0, len(plan.Hosts.Elements()))
	diags = plan.Hosts.ElementsAs(ctx, &hosts, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var updatedHosts []AapHost

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
			Inventory:   plan.ID.ValueInt64(),
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
				"Could not generate request body to update AAP host, unexpected error: "+err.Error(),
			)
			return
		}

		hostId := hostResource.Id.ValueInt64()
		var updatedHost *AapHost

		// If host is not in current inventory hosts create it, otherwise update it
		hostIndex := slices.IndexFunc(currentHosts, func(h AapHost) bool { return h.Id == hostId })
		if hostIndex == -1 {
			updatedHost, err = r.client.CreateHost(&buf)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error creating AAP host",
					"Could not create AAP host "+host.Name+", unexpected error: "+err.Error(),
				)
				return
			}
		} else {
			updatedHost, err = r.client.UpdateHost(strconv.Itoa(int(hostId)), &buf)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error updating AAP host",
					"Could not update AAP host "+host.Name+", unexpected error: "+err.Error(),
				)
				return
			}
		}

		// Update host struct values for later reference
		host.Id = updatedHost.Id

		hostGroups := make([]string, 0, len(hostResource.Groups.Elements()))
		diags = hostResource.Groups.ElementsAs(ctx, &hostGroups, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		host.Groups = hostGroups

		updatedHosts = append(updatedHosts, host)
	}

	// If any current inventory hosts are not in updated plan hosts, delete them
	for _, currentHost := range currentHosts {
		hostIndex := slices.IndexFunc(updatedHosts, func(h AapHost) bool { return h.Id == currentHost.Id })
		if hostIndex == -1 {
			hostId := strconv.Itoa(int(currentHost.Id))
			err = r.client.DeleteHost(hostId)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error deleting group",
					"Could not delete group "+currentHost.Name+", unexpected error: "+err.Error(),
				)
				return
			}
		}
	}

	// Ensure all hosts have updated groups
	for _, host := range updatedHosts {
		hostId := strconv.Itoa(int(host.Id))

		// Get hosts's current groups
		currentHostGroups, err := r.client.GetHostGroups(hostId)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error retrieving current groups for host",
				"Could not retrieve current groups for host "+host.Name+", unexpected error: "+err.Error(),
			)
			return
		}

		// If any updated host groups are not in current host groups, add them to host
		for _, groupName := range host.Groups {
			groupIndex := slices.IndexFunc(currentHostGroups, func(g AapGroup) bool { return g.Name == groupName })

			if groupIndex == -1 {
				hostGroupId, err := GroupIdFromName(groupName, updatedGroups)
				if err != nil {
					resp.Diagnostics.AddError(
						"Error retrieving group ID",
						"Could not retrieve ID for host group "+groupName+", unexpected error: "+err.Error(),
					)
					return
				}

				err = r.client.AddGroupToHost(hostId, hostGroupId)
				if err != nil {
					resp.Diagnostics.AddError(
						"Error adding group to host",
						"Could not add group "+groupName+" to host "+host.Name+", unexpected error: "+err.Error(),
					)
					return
				}
			}
		}

		// If any current host groups are not in updated host groups, remove them from host
		for _, hostGroup := range currentHostGroups {
			containsGroup := slices.Contains(host.Groups, hostGroup.Name)
			if !containsGroup {
				err = r.client.RemoveGroupFromHost(hostId, hostGroup.Id)
				if err != nil {
					resp.Diagnostics.AddError(
						"Error removing group from host",
						"Could not remove group "+hostGroup.Name+" from host "+host.Name+", unexpected error: "+err.Error(),
					)
					return
				}
			}
		}
	}

	// 	Map updated hosts back to schema and update state
	schemaHosts, diags := HostsToSchema(ctx, updatedHosts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	updatedStateHosts, diags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: hostTypes}, schemaHosts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Hosts = updatedStateHosts

	// Set state to fully populated inventory data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *aapInventoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Get current state
	var state aapInventoryResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete inventory from AAP
	err := r.client.DeleteInventory(state.ID.String())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting AAP inventory",
			"Could not delete AAP inventory with ID "+state.ID.String()+": "+err.Error(),
		)
		return
	}
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
