package resources

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hevoio/hevo-go"
	"github.com/hevoio/hevo-go/pipelines"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core/model"
	hevoschema "github.com/hevoio/terraform-provider-hevo/hevo/framework/core/schema"
)

const defaultPipelineTimeout = 20 * time.Minute

// Pipeline is the hevo_pipeline resource.
func Pipeline() resource.Resource {
	return &pipeline{}
}

type pipeline struct {
	core.ProviderResource
}

var (
	_ resource.ResourceWithConfigure   = (*pipeline)(nil)
	_ resource.ResourceWithImportState = (*pipeline)(nil)
	_ resource.ResourceWithModifyPlan  = (*pipeline)(nil)
)

func (r *pipeline) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pipeline"
}

func (r *pipeline) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = hevoschema.PipelineResourceSchema(ctx)
}

func (r *pipeline) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if _, err := strconv.ParseInt(req.ID, 10, 64); err != nil {
		resp.Diagnostics.AddError("Invalid import id", fmt.Sprintf("Import a Pipeline by its numeric id; got %q.", req.ID))

		return
	}

	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ModifyPlan warns when the plan replaces the Pipeline, and checks source_config against the
// connector's schema unless the provider skips plan-time validation.
func (r *pipeline) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}

	if !req.State.Raw.IsNull() && len(resp.RequiresReplace) > 0 {
		resp.Diagnostics.AddWarning("The Pipeline will be replaced",
			"A change to source_connector_id, destination_id, destination_prefix or replication_type creates a new "+
				"Pipeline. The new Pipeline loads all historical data again, and the old one is deleted.")
	}

	if r.GetClient() == nil || r.GetSkipPlanTimeValidation() {
		return
	}

	var plan model.PipelineResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() || plan.SourceConnectorID.IsUnknown() || !core.IsFullyKnown(plan.SourceConfig) {
		return
	}

	config, diags := core.DynamicToMap(plan.SourceConfig)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	schema, err := core.GetSourceConnectorSchema(ctx, r.GetClient(), r.GetMetadataCache(), plan.SourceConnectorID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("source_connector_id"), "Unable to read the connector's schema",
			fmt.Sprintf("%v. Set skip_plan_time_validation = true in the provider block to plan without it; "+
				"config mistakes then surface at apply.", err))

		return
	}

	core.ValidateConfig(config, schema, path.Root("source_config"), &resp.Diagnostics)
}

func (r *pipeline) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.GetClient() == nil {
		core.AddUnconfiguredClientError(&resp.Diagnostics)

		return
	}

	var plan model.PipelineResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := plan.Timeouts.Create(ctx, defaultPipelineTimeout)
	resp.Diagnostics.Append(diags...)

	call, diags := r.createCall(plan)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	created, err := call.Do(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Pipeline", err.Error())

		return
	}

	current := created.Data
	if core.IsTransitionalPipelineState(current.State) {
		if current, err = core.WaitForPipeline(ctx, r.GetClient(), current.ID); err != nil {
			resp.Diagnostics.AddError("Pipeline created but not settled", err.Error())
		}
	}

	plan.ReadFromResponse(current)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pipeline) createCall(plan model.PipelineResourceModel) (*pipelines.CreatePipelineService, diag.Diagnostics) {
	config, diags := core.DynamicToMap(plan.SourceConfig)

	auth, d := sourceAuth(plan.SourceAuth)
	diags.Append(d...)

	destinationID, err := strconv.ParseInt(plan.DestinationID.ValueString(), 10, 64)
	if err != nil {
		diags.AddAttributeError(path.Root("destination_id"), "Invalid destination_id", err.Error())
	}

	source := hevo.NewSource().
		Name(plan.SourceName.ValueString()).
		ConnectorID(plan.SourceConnectorID.ValueString()).
		Config(config)
	if auth != nil {
		source.Auth(auth)
	}

	call := r.GetClient().CreatePipeline().
		Name(plan.Name.ValueString()).
		Source(source).
		DestinationID(destinationID).
		DestinationPrefix(plan.DestinationPrefix.ValueString()).
		ReplicationType(plan.ReplicationType.ValueString()).
		LoadMode(plan.LoadMode.ValueString()).
		SchemaEvolution(plan.SchemaEvolution.ValueString()).
		Schedule(schedule(plan.Schedule)).
		FailureHandlingPolicy(failureHandlingPolicy(plan.FailureHandlingPolicy)).
		// Always created paused; hevo_pipeline_state enables it once its objects are selected.
		CreateDisabled(true)

	if plan.LatencyAlert != nil {
		call.LatencyAlert(latencyAlert(plan.LatencyAlert))
	}

	return call, diags
}

func (r *pipeline) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.GetClient() == nil {
		core.AddUnconfiguredClientError(&resp.Diagnostics)

		return
	}

	var state model.PipelineResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id, err := state.PipelineID()
	if err != nil {
		resp.Diagnostics.AddError("Invalid Pipeline id in state", err.Error())

		return
	}

	current, err := r.GetClient().GetPipeline().PipelineID(id).Do(ctx)
	if hevo.IsNotFound(err) {
		resp.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		resp.Diagnostics.AddError("Unable to read Pipeline", err.Error())

		return
	}

	// source_config and source_auth are never returned: they keep their state values.
	state.ReadFromResponse(current.Data)

	if state.SourceAuth.IsUnknown() {
		state.SourceAuth = types.DynamicNull()
	}

	if state.SourceConfig.IsUnknown() {
		state.SourceConfig = types.DynamicNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *pipeline) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.GetClient() == nil {
		core.AddUnconfiguredClientError(&resp.Diagnostics)

		return
	}

	var plan, state model.PipelineResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := plan.Timeouts.Update(ctx, defaultPipelineTimeout)
	resp.Diagnostics.Append(diags...)

	id, err := state.PipelineID()
	if err != nil {
		resp.Diagnostics.AddError("Invalid Pipeline id in state", err.Error())
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if call := r.settingsUpdate(plan, state, id); call != nil {
		if !r.apply(ctx, id, "settings", func(ctx context.Context) error { _, err := call.Do(ctx); return err }, &resp.Diagnostics) {
			return
		}
	}

	call, diags := r.sourceUpdate(plan, state, id)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	if call != nil {
		if !r.apply(ctx, id, "Source", func(ctx context.Context) error { _, err := call.Do(ctx); return err }, &resp.Diagnostics) {
			return
		}
	}

	current, err := r.GetClient().GetPipeline().PipelineID(id).Do(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Pipeline after update", err.Error())

		return
	}

	plan.ReadFromResponse(current.Data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// apply sends one edit, retrying while the Pipeline is busy, then waits for Hevo to apply it.
func (r *pipeline) apply(ctx context.Context, id int64, what string, send func(context.Context) error, diags *diag.Diagnostics) bool {
	if err := core.RetryOnConflict(ctx, send); err != nil {
		diags.AddError("Unable to update the Pipeline's "+what, err.Error())

		return false
	}

	if _, err := core.WaitForPipeline(ctx, r.GetClient(), id); err != nil {
		diags.AddError("The Pipeline's "+what+" update did not finish", err.Error())

		return false
	}

	return true
}

// settingsUpdate builds the PATCH of the Pipeline's own settings, or nil when none changed.
func (r *pipeline) settingsUpdate(plan, state model.PipelineResourceModel, id int64) *pipelines.UpdatePipelineService {
	call := r.GetClient().UpdatePipeline().PipelineID(id)
	hasChange := false

	if !plan.Name.Equal(state.Name) {
		call.Name(plan.Name.ValueString())

		hasChange = true
	}

	if !plan.LoadMode.Equal(state.LoadMode) {
		call.LoadMode(plan.LoadMode.ValueString())

		hasChange = true
	}

	if !plan.SchemaEvolution.Equal(state.SchemaEvolution) {
		call.SchemaEvolution(plan.SchemaEvolution.ValueString())

		hasChange = true
	}

	if !equalSchedule(plan.Schedule, state.Schedule) {
		call.Schedule(schedule(plan.Schedule))

		hasChange = true
	}

	if !equalFailurePolicy(plan.FailureHandlingPolicy, state.FailureHandlingPolicy) {
		call.FailureHandlingPolicy(failureHandlingPolicy(plan.FailureHandlingPolicy))

		hasChange = true
	}

	if plan.LatencyAlert != nil && !equalLatencyAlert(plan.LatencyAlert, state.LatencyAlert) {
		call.LatencyAlert(latencyAlert(plan.LatencyAlert))

		hasChange = true
	}

	if !hasChange {
		return nil
	}

	return call
}

// sourceUpdate builds the PATCH of the Source, or nil when nothing changed. Config holds only the
// keys that changed; after an import, when state has no config, it holds all of them.
func (r *pipeline) sourceUpdate(plan, state model.PipelineResourceModel, id int64) (*pipelines.UpdatePipelineSourceService, diag.Diagnostics) {
	planConfig, diags := core.DynamicToMap(plan.SourceConfig)
	stateConfig, d := core.DynamicToMap(state.SourceConfig)
	diags.Append(d...)

	call := r.GetClient().UpdatePipelineSource().PipelineID(id)
	hasChange := false

	if !plan.SourceName.Equal(state.SourceName) {
		call.Name(plan.SourceName.ValueString())

		hasChange = true
	}

	if changed := core.ChangedConfig(planConfig, stateConfig); len(changed) > 0 {
		call.Config(changed)

		hasChange = true
	}

	if !plan.SourceAuth.Equal(state.SourceAuth) && !plan.SourceAuth.IsNull() {
		auth, d := sourceAuth(plan.SourceAuth)
		diags.Append(d...)
		call.Auth(auth)

		hasChange = true
	}

	if !hasChange {
		return nil, diags
	}

	return call, diags
}

func (r *pipeline) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.GetClient() == nil {
		core.AddUnconfiguredClientError(&resp.Diagnostics)

		return
	}

	var state model.PipelineResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := state.Timeouts.Delete(ctx, defaultPipelineTimeout)
	resp.Diagnostics.Append(diags...)

	id, err := state.PipelineID()
	if err != nil {
		resp.Diagnostics.AddError("Invalid Pipeline id in state", err.Error())
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err = core.RetryOnConflict(ctx, func(ctx context.Context) error {
		_, err := r.GetClient().DeletePipeline().PipelineID(id).Do(ctx)

		return err
	})
	if err != nil && !hevo.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Pipeline", err.Error())
	}
}
