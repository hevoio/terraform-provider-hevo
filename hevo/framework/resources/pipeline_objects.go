package resources

import (
	"context"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hevoio/hevo-go"
	pipelineobjects "github.com/hevoio/hevo-go/pipeline_objects"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core/model"
	hevoschema "github.com/hevoio/terraform-provider-hevo/hevo/framework/core/schema"
)

const objectsPageLimit = 500

// PipelineObjects is the hevo_pipeline_objects resource.
func PipelineObjects() resource.Resource {
	return &pipelineObjects{}
}

type pipelineObjects struct {
	core.ProviderResource
}

var (
	_ resource.ResourceWithConfigure   = (*pipelineObjects)(nil)
	_ resource.ResourceWithImportState = (*pipelineObjects)(nil)
)

func (r *pipelineObjects) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pipeline_objects"
}

func (r *pipelineObjects) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = hevoschema.PipelineObjectsResourceSchema(ctx)
}

func (r *pipelineObjects) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByPipelineID(ctx, req, resp)
}

func (r *pipelineObjects) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model.PipelineObjectsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := plan.Timeouts.Create(ctx, defaultPipelineTimeout)
	resp.Diagnostics.Append(diags...)

	r.apply(ctx, timeout, plan, nil, &resp.Diagnostics)

	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = plan.PipelineID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pipelineObjects) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state model.PipelineObjectsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := plan.Timeouts.Update(ctx, defaultPipelineTimeout)
	resp.Diagnostics.Append(diags...)

	r.apply(ctx, timeout, plan, state.Objects, &resp.Diagnostics)

	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = plan.PipelineID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// apply sends the desired selection in one request, retrying while the Pipeline is busy, then
// waits for Hevo to apply it.
func (r *pipelineObjects) apply(
	ctx context.Context,
	timeout time.Duration,
	plan model.PipelineObjectsResourceModel,
	prior map[string]model.PipelineObjectModel,
	diags *diag.Diagnostics,
) {
	if r.GetClient() == nil {
		core.AddUnconfiguredClientError(diags)

		return
	}

	id, err := strconv.ParseInt(plan.PipelineID.ValueString(), 10, 64)
	if err != nil {
		diags.AddAttributeError(path.Root("pipeline_id"), "Invalid pipeline_id", err.Error())

		return
	}

	call := r.GetClient().UpdatePipelineObjects().PipelineID(id)
	for _, selection := range selections(ctx, plan, prior, diags) {
		call.Object(selection)
	}

	if plan.Select.ValueString() == hevoschema.SelectAll {
		call.SelectionMode(pipelineobjects.SelectionModeIncludeAll)
	} else {
		call.SelectionMode(pipelineobjects.SelectionModeExcludeAll)
	}

	if diags.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err = core.RetryOnConflict(ctx, func(ctx context.Context) error {
		_, err := call.Do(ctx)

		return err
	})
	if err != nil {
		diags.AddError("Unable to update the Pipeline's objects", err.Error())

		return
	}

	if _, err := core.WaitForPipeline(ctx, r.GetClient(), id); err != nil {
		diags.AddError("The object selection did not finish applying", err.Error())
	}
}

// selections builds one entry per object to send. Under "listed" (EXCLUDE_ALL) every enabled
// object is named, so the request leaves exactly those selected; under "all" (INCLUDE_ALL) the
// disabled objects are named, plus enabled ones that carry a load mode or excluded fields.
// Fields excluded before and no longer excluded are sent as included.
func selections(
	ctx context.Context,
	plan model.PipelineObjectsResourceModel,
	prior map[string]model.PipelineObjectModel,
	diags *diag.Diagnostics,
) []*pipelineobjects.ObjectSelection {
	listed := plan.Select.ValueString() == hevoschema.SelectListed
	names := make([]string, 0, len(plan.Objects))

	for name := range plan.Objects {
		names = append(names, name)
	}

	sort.Strings(names)

	result := []*pipelineobjects.ObjectSelection{}

	for _, name := range names {
		object := plan.Objects[name]
		enabled := object.Enabled.IsNull() || object.Enabled.ValueBool()
		hasOverrides := !object.LoadMode.IsNull() || !object.ExcludedFields.IsNull()

		if listed && !enabled || !listed && enabled && !hasOverrides {
			continue
		}

		selection := hevo.NewObjectSelection().Name(name).Selected(enabled)
		if enabled {
			addOverrides(ctx, selection, object, prior[name], diags)
		}

		result = append(result, selection)
	}

	return result
}

func addOverrides(
	ctx context.Context,
	selection *pipelineobjects.ObjectSelection,
	object, prior model.PipelineObjectModel,
	diags *diag.Diagnostics,
) {
	if !object.LoadMode.IsNull() {
		selection.LoadMode(object.LoadMode.ValueString())
	}

	excluded := stringSet(ctx, object.ExcludedFields, diags)
	if len(excluded) > 0 {
		selection.ExcludeFields(excluded...)
	}

	var included []string

	for _, field := range stringSet(ctx, prior.ExcludedFields, diags) {
		if !contains(excluded, field) {
			included = append(included, field)
		}
	}

	if len(included) > 0 {
		selection.IncludeFields(included...)
	}
}

func (r *pipelineObjects) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.GetClient() == nil {
		core.AddUnconfiguredClientError(&resp.Diagnostics)

		return
	}

	var state model.PipelineObjectsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	id, err := strconv.ParseInt(state.PipelineID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid pipeline_id in state", err.Error())
	}

	if resp.Diagnostics.HasError() {
		return
	}

	objects, err := r.listObjects(ctx, id)
	if hevo.IsNotFound(err) {
		// The Pipeline is gone, and its objects with it.
		resp.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		resp.Diagnostics.AddError("Unable to read the Pipeline's objects", err.Error())

		return
	}

	resp.Diagnostics.Append(state.ReadFromObjects(ctx, objects)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *pipelineObjects) listObjects(ctx context.Context, pipelineID int64) ([]pipelineobjects.PipelineObject, error) {
	all := []pipelineobjects.PipelineObject{}
	cursor := ""

	for {
		call := r.GetClient().ListPipelineObjects().PipelineID(pipelineID).Limit(objectsPageLimit)
		if cursor != "" {
			call.Cursor(cursor)
		}

		page, err := call.Do(ctx)
		if err != nil {
			return nil, err
		}

		all = append(all, page.Data...)

		if !page.HasMore || page.NextCursor == "" {
			return all, nil
		}

		cursor = page.NextCursor
	}
}

// Delete leaves the selection as it is, like removing any other part of a Pipeline's
// configuration from Terraform.
func (r *pipelineObjects) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {}

func stringSet(ctx context.Context, set types.Set, diags *diag.Diagnostics) []string {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}

	var values []string

	diags.Append(set.ElementsAs(ctx, &values, false)...)
	sort.Strings(values)

	return values
}

func contains(values []string, v string) bool {
	return slices.Contains(values, v)
}
