// Package resources holds the provider's resources, one file each.
package resources

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hevoio/hevo-go"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core/model"
	hevoschema "github.com/hevoio/terraform-provider-hevo/hevo/framework/core/schema"
)

// Destination is the hevo_destination resource.
func Destination() resource.Resource {
	return &destination{}
}

type destination struct {
	core.ProviderResource
}

var (
	_ resource.ResourceWithConfigure   = (*destination)(nil)
	_ resource.ResourceWithImportState = (*destination)(nil)
	_ resource.ResourceWithModifyPlan  = (*destination)(nil)
)

func (r *destination) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_destination"
}

func (r *destination) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = hevoschema.DestinationResourceSchema()
}

func (r *destination) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if _, err := strconv.ParseInt(req.ID, 10, 64); err != nil {
		resp.Diagnostics.AddError("Invalid import id", fmt.Sprintf("Import a Destination by its numeric id; got %q.", req.ID))

		return
	}

	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ModifyPlan checks config against the connector's schema, unless the provider skips plan-time
// validation or the config is not known yet.
func (r *destination) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || r.GetClient() == nil || r.GetSkipPlanTimeValidation() {
		return
	}

	var plan model.DestinationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() || plan.ConnectorID.IsUnknown() || !core.IsFullyKnown(plan.Config) {
		return
	}

	config, diags := core.DynamicToMap(plan.Config)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	schema, err := core.GetDestinationConnectorSchema(ctx, r.GetClient(), r.GetMetadataCache(), plan.ConnectorID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("connector_id"), "Unable to read the connector's schema",
			fmt.Sprintf("%v. Set skip_plan_time_validation = true in the provider block to plan without it; "+
				"config mistakes then surface at apply.", err))

		return
	}

	core.ValidateConfig(config, schema, path.Root("config"), &resp.Diagnostics)
}

func (r *destination) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.GetClient() == nil {
		core.AddUnconfiguredClientError(&resp.Diagnostics)

		return
	}

	var plan model.DestinationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	config, diags := core.DynamicToMap(plan.Config)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.GetClient().CreateDestination().
		Name(plan.Name.ValueString()).
		ConnectorID(plan.ConnectorID.ValueString()).
		Config(config).
		Do(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Destination", err.Error())

		return
	}

	// The planned config is what Terraform expects back; Hevo masks credentials on read anyway.
	plan.ReadFromResponse(created.Data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *destination) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.GetClient() == nil {
		core.AddUnconfiguredClientError(&resp.Diagnostics)

		return
	}

	var state model.DestinationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := state.DestinationID()
	if err != nil {
		resp.Diagnostics.AddError("Invalid Destination id in state", err.Error())

		return
	}

	current, err := r.GetClient().GetDestination().DestinationID(id).Do(ctx)
	if hevo.IsNotFound(err) {
		// Deleted outside Terraform: drop it so the next plan recreates it.
		resp.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		resp.Diagnostics.AddError("Unable to read Destination", err.Error())

		return
	}

	// Without the schema, masked values are still recognised; only drift on write-only keys
	// that Hevo returns unmasked would be missed.
	schema, _ := core.GetDestinationConnectorSchema(ctx, r.GetClient(), r.GetMetadataCache(), current.Data.ConnectorID)

	state.ReadFromResponse(current.Data)
	resp.Diagnostics.Append(state.ReadConfigFromResponse(current.Data, schema)...)

	if state.ForceDelete.IsNull() {
		state.ForceDelete = boolFalse()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *destination) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.GetClient() == nil {
		core.AddUnconfiguredClientError(&resp.Diagnostics)

		return
	}

	var plan, state model.DestinationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	planConfig, diags := core.DynamicToMap(plan.Config)
	resp.Diagnostics.Append(diags...)
	stateConfig, diags := core.DynamicToMap(state.Config)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := state.DestinationID()
	if err != nil {
		resp.Diagnostics.AddError("Invalid Destination id in state", err.Error())

		return
	}

	call := r.GetClient().UpdateDestination().DestinationID(id)
	hasChange := false

	if !plan.Name.Equal(state.Name) {
		call.Name(plan.Name.ValueString())

		hasChange = true
	}

	if changed := core.ChangedConfig(planConfig, stateConfig); len(changed) > 0 {
		call.Config(changed)

		hasChange = true
	}

	// A change to force_delete alone only updates state.
	if hasChange {
		updated, err := call.Do(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Unable to update Destination", err.Error())

			return
		}

		plan.ReadFromResponse(updated.Data)
	} else {
		plan.CopyComputed(state)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *destination) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.GetClient() == nil {
		core.AddUnconfiguredClientError(&resp.Diagnostics)

		return
	}

	var state model.DestinationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := state.DestinationID()
	if err != nil {
		resp.Diagnostics.AddError("Invalid Destination id in state", err.Error())

		return
	}

	call := r.GetClient().DeleteDestination().DestinationID(id)
	if state.ForceDelete.ValueBool() {
		call.ConfirmOverride(true)
	}

	_, err = call.Do(ctx)

	switch {
	case err == nil, hevo.IsNotFound(err):
		return
	case hevo.IsConflict(err):
		resp.Diagnostics.AddError("Destination still has Pipelines",
			err.Error()+"\n\nDelete those Pipelines first, or set force_delete = true on this Destination, "+
				"apply, and destroy again to delete them with it.")
	default:
		resp.Diagnostics.AddError("Unable to delete Destination", err.Error())
	}
}
