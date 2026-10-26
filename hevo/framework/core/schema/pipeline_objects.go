package schema

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Values of hevo_pipeline_objects.select.
const (
	SelectAll    = "all"
	SelectListed = "listed"
)

// PipelineObjectsResourceSchema is the schema of hevo_pipeline_objects.
func PipelineObjectsResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "The Source objects a Pipeline replicates, as a desired set: with `select = \"listed\"` " +
			"exactly the enabled objects in `objects` are replicated; with `select = \"all\"` every object is, " +
			"except those disabled in `objects`. Objects selected or deselected outside Terraform show as drift. " +
			"Removing this resource leaves the selection as it is.",
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
			"select": schema.StringAttribute{
				Required: true,
				Description: "`listed` replicates only the objects enabled in `objects`; `all` replicates every " +
					"object except those disabled in `objects`.",
				Validators: []validator.String{stringvalidator.OneOf(SelectAll, SelectListed)},
			},
			"objects": schema.MapNestedAttribute{
				Optional: true,
				Description: "Objects keyed by fully qualified name, as the Pipeline lists them in " +
					"`fully_qualified_name` (for example `public.orders`). A child object follows its parent; name " +
					"only the parent.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"enabled": schema.BoolAttribute{
							Optional:    true,
							Computed:    true,
							Default:     booldefault.StaticBool(true),
							Description: "Whether the object is replicated. Defaults to true.",
						},
						"load_mode": schema.StringAttribute{
							Optional:    true,
							Description: "This object's load mode, instead of the Pipeline's: `MERGE`, `APPEND` or `HISTORY_TRACKING`.",
							Validators:  []validator.String{stringvalidator.OneOf("MERGE", "APPEND", "HISTORY_TRACKING")},
						},
						"excluded_fields": schema.SetAttribute{
							Optional:    true,
							ElementType: types.StringType,
							Description: "Fields of this object not to replicate.",
						},
					},
				},
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true}),
		},
	}
}
