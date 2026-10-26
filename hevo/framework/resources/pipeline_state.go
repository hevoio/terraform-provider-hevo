package resources

import (
	"context"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hevoio/hevo-go"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core/model"
	hevoschema "github.com/hevoio/terraform-provider-hevo/hevo/framework/core/schema"
)

// PipelineState is the hevo_pipeline_state resource.
func PipelineState() resource.Resource {
	return &pipelineState{}
}

type pipelineState struct {
	core.ProviderResource
}

var (
	_ resource.ResourceWithConfigure   = (*pipelineState)(nil)
	_ resource.ResourceWithImportState = (*pipelineState)(nil)
)

func (r *pipelineState) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pipeline_state"
}

func (r *pipelineState) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = hevoschema.PipelineStateResourceSchema(ctx)
}

func (r *pipelineState) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByPipelineID(ctx, req, resp)
}

func (r *pipelineState) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model.PipelineStateResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	timeout, diags := plan.Timeouts.Create(ctx, defaultPipelineTimeout)
	resp.Diagnostics.Append(diags...)

	r.apply(ctx, timeout, plan, &resp.State, &resp.Diagnostics)
}

func (r *pipelineState) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan model.PipelineStateResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	timeout, diags := plan.Timeouts.Update(ctx, defaultPipelineTimeout)
	resp.Diagnostics.Append(diags...)

	r.apply(ctx, timeout, plan, &resp.State, &resp.Diagnostics)
}

// apply sets the Pipeline to the planned state and records it.
func (r *pipelineState) apply(
	ctx context.Context,
	timeout time.Duration,
	plan model.PipelineStateResourceModel,
	state *tfsdk.State,
	diags *diag.Diagnostics,
) {
	if diags.HasError() {
		return
	}

	r.setPaused(ctx, timeout, plan, diags)

	if diags.HasError() {
		return
	}

	plan.ID = plan.PipelineID
	diags.Append(state.Set(ctx, &plan)...)
}

// setPaused enables or disables the Pipeline unless it is already so, retrying while it is busy,
// and waits for it to settle.
func (r *pipelineState) setPaused(ctx context.Context, timeout time.Duration, plan model.PipelineStateResourceModel, diags *diag.Diagnostics) {
	if r.GetClient() == nil {
		core.AddUnconfiguredClientError(diags)

		return
	}

	id, err := strconv.ParseInt(plan.PipelineID.ValueString(), 10, 64)
	if err != nil {
		diags.AddAttributeError(path.Root("pipeline_id"), "Invalid pipeline_id", err.Error())

		return
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Let an edit still being applied finish, so its transitional state is not mistaken for ours.
	current, err := core.WaitForPipeline(ctx, r.GetClient(), id)
	if err != nil {
		diags.AddError("Unable to read the Pipeline", err.Error())

		return
	}

	paused := plan.Paused.ValueBool()
	if current.Disabled == paused {
		return
	}

	action := "enable"
	send := func(ctx context.Context) error {
		_, err := r.GetClient().EnablePipeline().PipelineID(id).Do(ctx)

		return err
	}

	if paused {
		action = "disable"
		send = func(ctx context.Context) error {
			_, err := r.GetClient().DisablePipeline().PipelineID(id).Do(ctx)

			return err
		}
	}

	if err := core.RetryOnConflict(ctx, send); err != nil {
		diags.AddError("Unable to "+action+" the Pipeline", err.Error())

		return
	}

	if _, err := core.WaitForPipeline(ctx, r.GetClient(), id); err != nil {
		diags.AddError("The Pipeline did not finish trying to "+action, err.Error())
	}
}

func (r *pipelineState) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.GetClient() == nil {
		core.AddUnconfiguredClientError(&resp.Diagnostics)

		return
	}

	var state model.PipelineStateResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	id, err := strconv.ParseInt(state.PipelineID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid pipeline_id in state", err.Error())
	}

	if resp.Diagnostics.HasError() {
		return
	}

	current, err := r.GetClient().GetPipeline().PipelineID(id).Do(ctx)
	if hevo.IsNotFound(err) {
		resp.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		resp.Diagnostics.AddError("Unable to read the Pipeline", err.Error())

		return
	}

	state.Paused = types.BoolValue(current.Data.Disabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete leaves the Pipeline enabled or paused as it is.
func (r *pipelineState) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {}
