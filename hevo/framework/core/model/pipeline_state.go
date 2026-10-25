package model

import (
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// PipelineStateResourceModel is the Terraform value of hevo_pipeline_state.
type PipelineStateResourceModel struct {
	ID         types.String   `tfsdk:"id"`
	PipelineID types.String   `tfsdk:"pipeline_id"`
	Paused     types.Bool     `tfsdk:"paused"`
	Timeouts   timeouts.Value `tfsdk:"timeouts"`
}
