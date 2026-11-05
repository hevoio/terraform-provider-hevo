package schema

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// PipelineObjectsDatasourceSchema is the schema of the hevo_pipeline_objects data source.
func PipelineObjectsDatasourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Lists a Pipeline's objects with the names `hevo_pipeline_objects` takes. By default only " +
			"objects whose selection can change are listed; a source can have tens of thousands of others.",
		Attributes: map[string]schema.Attribute{
			"pipeline_id": schema.StringAttribute{
				Required:    true,
				Description: "The Pipeline, such as `hevo_pipeline.x.id`.",
			},
			"statuses": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Keep only objects in these statuses. Defaults to the ones whose selection can change: " +
					"`ACTIVE`, `INACTIVE`, `RESETTING`, `BLOCKED`, `INCONSISTENT`.",
			},
			"objects": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The objects.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"fully_qualified_name": computedString("The name `hevo_pipeline_objects` takes, for example " +
							"`shop.public.orders` for Postgres."),
						"name":      computedString("The object's own name."),
						"status":    computedString("For example `ACTIVE` (selected) or `INACTIVE` (not selected)."),
						"selected":  schema.BoolAttribute{Computed: true, Description: "Whether it is replicated."},
						"load_mode": computedString("`MERGE`, `APPEND` or `HISTORY_TRACKING`."),
					},
				},
			},
		},
	}
}
