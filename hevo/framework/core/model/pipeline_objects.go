package model

import (
	"context"
	"slices"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pipelineobjects "github.com/hevoio/hevo-go/pipeline_objects"
)

// Object statuses under which Hevo lets an object be selected or deselected.
var selectableObjectStatuses = []string{"ACTIVE", "INACTIVE", "RESETTING", "BLOCKED", "INCONSISTENT"}

// PipelineObjectsResourceModel is the Terraform value of hevo_pipeline_objects.
type PipelineObjectsResourceModel struct {
	ID         types.String                   `tfsdk:"id"`
	PipelineID types.String                   `tfsdk:"pipeline_id"`
	Select     types.String                   `tfsdk:"select"`
	Objects    map[string]PipelineObjectModel `tfsdk:"objects"`
	Timeouts   timeouts.Value                 `tfsdk:"timeouts"`
}

// PipelineObjectModel is one object's desired selection.
type PipelineObjectModel struct {
	Enabled        types.Bool   `tfsdk:"enabled"`
	LoadMode       types.String `tfsdk:"load_mode"`
	ExcludedFields types.Set    `tfsdk:"excluded_fields"`
}

// ObjectKey is how an object is named in hevo_pipeline_objects and in a selection: its fully
// qualified name (schema.object), or its name for a source without schemas.
func ObjectKey(o pipelineobjects.PipelineObject) string {
	if o.FullyQualifiedName != "" {
		return o.FullyQualifiedName
	}

	return o.Name
}

// IsSelectable reports whether an object may be selected or deselected, so whether it counts
// toward drift. A child object follows its parent and never counts on its own.
func IsSelectable(o pipelineobjects.PipelineObject) bool {
	return o.ParentObject == "" && slices.Contains(selectableObjectStatuses, o.Status)
}

// ReadFromObjects sets Objects from the Pipeline's objects. Each object in state takes its remote
// selection, and its load mode and excluded fields when the user set them. An object outside
// state that departs from the desired set is added, so the plan shows it as drift: under
// "listed" one selected outside Terraform, under "all" one deselected outside Terraform. On
// import (no select in state) the selection is read as "listed".
func (m *PipelineObjectsResourceModel) ReadFromObjects(ctx context.Context, remote []pipelineobjects.PipelineObject) diag.Diagnostics {
	var diags diag.Diagnostics

	if m.Select.IsNull() || m.Select.IsUnknown() {
		m.Select = types.StringValue("listed")
	}

	listed := m.Select.ValueString() == "listed"
	result := map[string]PipelineObjectModel{}

	for _, o := range remote {
		key := ObjectKey(o)
		prior, inState := m.Objects[key]

		switch {
		case inState:
			current := PipelineObjectModel{Enabled: types.BoolValue(o.Selected), LoadMode: types.StringNull(), ExcludedFields: prior.ExcludedFields}
			if !prior.LoadMode.IsNull() {
				current.LoadMode = types.StringValue(o.LoadMode)
			}

			if !prior.ExcludedFields.IsNull() {
				set, d := types.SetValueFrom(ctx, types.StringType, excludedFields(o))
				diags.Append(d...)

				current.ExcludedFields = set
			}

			result[key] = current
		case !IsSelectable(o):
		case listed && o.Selected, !listed && !o.Selected:
			result[key] = PipelineObjectModel{
				Enabled:        types.BoolValue(o.Selected),
				LoadMode:       types.StringNull(),
				ExcludedFields: types.SetNull(types.StringType),
			}
		}
	}

	if len(result) == 0 && m.Objects == nil {
		result = nil
	}

	m.Objects = result

	return diags
}

// excludedFields lists an object's selectable fields that are not replicated, sorted.
func excludedFields(o pipelineobjects.PipelineObject) []string {
	fields := []string{}

	for _, f := range o.Fields {
		if f.Selectable && !f.Selected {
			fields = append(fields, f.SourceName)
		}
	}

	sort.Strings(fields)

	return fields
}
