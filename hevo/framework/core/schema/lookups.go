package schema

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func computedString(description string) schema.StringAttribute {
	return schema.StringAttribute{Computed: true, Description: description}
}

func destinationAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":         computedString("The Destination's display name."),
		"connector_id": computedString("The destination connector, for example `snowflake`."),
		"type":         computedString("The destination type, for example `SNOWFLAKE`."),
		"status":       computedString("The Destination's status, for example `CREATED`."),
		"created_at":   computedString("When the Destination was created, in RFC 3339."),
		"created_by":   computedString("Who created the Destination."),
		"updated_at":   computedString("When the Destination was last changed, in RFC 3339."),
		"updated_by":   computedString("Who last changed the Destination."),
	}
}

func pipelineAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name":                computedString("The Pipeline's display name."),
		"state":               computedString("The Pipeline's state, for example `Initialized` or `Disabled`."),
		"disabled":            schema.BoolAttribute{Computed: true, Description: "Whether the Pipeline is paused."},
		"health":              computedString("The Pipeline's health, for example `HEALTHY`."),
		"source_name":         computedString("The Source's display name."),
		"source_connector_id": computedString("The source connector, for example `hubspot`."),
		"source_type":         computedString("The source type, for example `HUBSPOT`."),
		"destination_id":      computedString("The id of the Destination the Pipeline loads into."),
		"destination_prefix":  computedString("The prefix of the tables the Pipeline creates."),
		"replication_type":    computedString("`HISTORICAL_AND_INCREMENTAL` or `INCREMENTAL_ONLY`."),
		"load_mode":           computedString("`MERGE`, `APPEND` or `HISTORY_TRACKING`."),
		"schema_evolution":    computedString("`ALLOW_ALL`, `BLOCK_ALL` or `ALLOW_COLUMN_LEVEL`."),
		"sync_type":           computedString("The schedule: `SCHEDULED` or `ON_DEMAND`."),
		"frequency_minutes":   schema.Int64Attribute{Computed: true, Description: "How often a `SCHEDULED` Pipeline ingests."},
		"created_at":          computedString("When the Pipeline was created, in RFC 3339."),
		"created_by":          computedString("Who created the Pipeline."),
		"updated_at":          computedString("When the Pipeline was last changed, in RFC 3339."),
		"updated_by":          computedString("Who last changed the Pipeline."),
	}
}

func withID(attrs map[string]schema.Attribute, id schema.Attribute) map[string]schema.Attribute {
	attrs["id"] = id
	return attrs
}

// DestinationDatasourceSchema is the schema of the hevo_destination data source.
func DestinationDatasourceSchema() schema.Schema {
	attrs := withID(destinationAttributes(), schema.StringAttribute{Required: true, Description: "The Destination's id."})
	attrs["config"] = schema.DynamicAttribute{
		Computed:    true,
		Description: "The connector's settings as Hevo returns them, with credentials masked.",
	}

	return schema.Schema{Description: "Reads one Destination.", Attributes: attrs}
}

// DestinationsDatasourceSchema is the schema of the hevo_destinations data source.
func DestinationsDatasourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Lists the team's Destinations, ordered by id.",
		Attributes: map[string]schema.Attribute{
			"destinations": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The Destinations. Read one with the `hevo_destination` data source for its config.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: withID(destinationAttributes(), computedString("The Destination's id.")),
				},
			},
		},
	}
}

// PipelineDatasourceSchema is the schema of the hevo_pipeline data source.
func PipelineDatasourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Reads one Pipeline.",
		Attributes:  withID(pipelineAttributes(), schema.StringAttribute{Required: true, Description: "The Pipeline's id."}),
	}
}

// PipelinesDatasourceSchema is the schema of the hevo_pipelines data source.
func PipelinesDatasourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Lists the team's Pipelines, optionally filtered.",
		Attributes: map[string]schema.Attribute{
			"states": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Keep only Pipelines in these states, for example `[\"Initialized\", \"Disabled\"]`.",
			},
			"source_connector_id": schema.StringAttribute{
				Optional:    true,
				Description: "Keep only Pipelines whose Source uses this connector.",
			},
			"destination_id": schema.StringAttribute{
				Optional:    true,
				Description: "Keep only Pipelines that load into this Destination.",
			},
			"pipelines": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The Pipelines.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: withID(pipelineAttributes(), computedString("The Pipeline's id.")),
				},
			},
		},
	}
}
