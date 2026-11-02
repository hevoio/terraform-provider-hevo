package model

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	pipelineobjects "github.com/hevoio/hevo-go/pipeline_objects"
)

// PipelineObjectsDatasourceModel is the Terraform value of the hevo_pipeline_objects data source.
type PipelineObjectsDatasourceModel struct {
	PipelineID types.String          `tfsdk:"pipeline_id"`
	Statuses   []types.String        `tfsdk:"statuses"`
	Objects    []PipelineObjectEntry `tfsdk:"objects"`
}

// PipelineObjectEntry is one object in the data source's list.
type PipelineObjectEntry struct {
	FullyQualifiedName types.String `tfsdk:"fully_qualified_name"`
	Name               types.String `tfsdk:"name"`
	Status             types.String `tfsdk:"status"`
	Selected           types.Bool   `tfsdk:"selected"`
	LoadMode           types.String `tfsdk:"load_mode"`
}

// NewPipelineObjectEntry maps one API object.
func NewPipelineObjectEntry(o pipelineobjects.PipelineObject) PipelineObjectEntry {
	return PipelineObjectEntry{
		FullyQualifiedName: types.StringValue(ObjectKey(o)),
		Name:               types.StringValue(o.Name),
		Status:             types.StringValue(o.Status),
		Selected:           types.BoolValue(o.Selected),
		LoadMode:           types.StringValue(o.LoadMode),
	}
}
