package schema

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Connector kinds the connector data sources accept.
const (
	ConnectorKindSource      = "source"
	ConnectorKindDestination = "destination"
)

func connectorKindAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Required:    true,
		Description: "`source` for the connectors a Pipeline's Source can use, `destination` for a Destination's.",
		Validators:  []validator.String{stringvalidator.OneOf(ConnectorKindSource, ConnectorKindDestination)},
	}
}

// ConnectorsDatasourceSchema is the schema of the hevo_connectors data source.
func ConnectorsDatasourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Lists the connectors Hevo offers for Sources or Destinations.",
		Attributes: map[string]schema.Attribute{
			"kind": connectorKindAttribute(),
			"type": schema.StringAttribute{
				Optional:    true,
				Description: "Keep only the source connector of this type, for example `HUBSPOT`. Sources only.",
			},
			"connectors": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The connectors, in the order Hevo lists them.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"connector_id": schema.StringAttribute{
							Computed:    true,
							Description: "The connector's id, as `connector_id` takes it, for example `hubspot`.",
						},
						"type": schema.StringAttribute{
							Computed:    true,
							Description: "The connector's type, for example `HUBSPOT`.",
						},
						"display_name": schema.StringAttribute{
							Computed:    true,
							Description: "The connector's name in the Hevo UI.",
						},
						"description": schema.StringAttribute{
							Computed:    true,
							Description: "What the connector connects to.",
						},
						"documentation_url": schema.StringAttribute{
							Computed:    true,
							Description: "The connector's documentation.",
						},
						"in_beta": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether the connector is in beta.",
						},
					},
				},
			},
		},
	}
}

// ConnectorMetadataDatasourceSchema is the schema of the hevo_connector_metadata data source.
func ConnectorMetadataDatasourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Reads one connector with the JSON Schema of the body that creates it: every `config` key, " +
			"its type, and whether it is required or a write-only credential.",
		Attributes: map[string]schema.Attribute{
			"kind": connectorKindAttribute(),
			"connector_id": schema.StringAttribute{
				Required:    true,
				Description: "The connector's id, for example `snowflake`.",
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "The connector's type, for example `SNOWFLAKE`.",
			},
			"display_name": schema.StringAttribute{
				Computed:    true,
				Description: "The connector's name in the Hevo UI.",
			},
			"schema_json": schema.StringAttribute{
				Computed: true,
				Description: "The JSON Schema (draft 2020-12) of the create body, as JSON. Read it with " +
					"`jsondecode(...)`; the connector's settings are under `properties.config`.",
			},
			"required_config_keys": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "The `config` keys always required. Keys required only under a choice are in `schema_json`.",
			},
			"write_only_config_keys": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "The `config` keys that are credentials: accepted on write, masked in every read.",
			},
		},
	}
}
