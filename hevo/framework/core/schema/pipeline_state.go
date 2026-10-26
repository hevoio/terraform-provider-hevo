package schema

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// PipelineStateResourceSchema is the schema of hevo_pipeline_state.
func PipelineStateResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "Whether a Pipeline replicates or is paused. Kept apart from `hevo_pipeline` so one apply " +
			"can create the Pipeline, select its objects with `hevo_pipeline_objects`, and then enable it. " +
			"Removing this resource leaves the Pipeline as it is.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The Pipeline's id.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"pipeline_id": schema.StringAttribute{
				Required:      true,
				Description:   "The Pipeline, such as `hevo_pipeline.x.id`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"paused": schema.BoolAttribute{
				Required: true,
				Description: "false enables the Pipeline; it needs at least one selected object. true pauses it; " +
					"jobs in progress finish first.",
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}
