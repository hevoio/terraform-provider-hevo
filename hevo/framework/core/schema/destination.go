// Package schema holds the Terraform schema of each resource and data source.
package schema

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// DestinationResourceSchema is the schema of hevo_destination.
func DestinationResourceSchema() schema.Schema {
	return schema.Schema{
		Description: "A Destination: the warehouse or database Pipelines load into. Hevo tests the " +
			"connection on create and on every config change, and refuses a Destination it cannot reach.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The Destination's id.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The Destination's display name, unique within the team.",
			},
			"connector_id": schema.StringAttribute{
				Required: true,
				Description: "The destination connector, for example `snowflake`. List them with the " +
					"`hevo_connectors` data source. Changing it replaces the Destination.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"config": schema.DynamicAttribute{
				Required: true,
				Description: "The connector's settings, as its schema in the `hevo_connector_metadata` data source " +
					"names them. Credentials are write-only: Hevo masks them in every read, so the provider keeps " +
					"the value you set. Only the keys you set are compared for drift.",
			},
			"force_delete": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "Delete the Destination even when Pipelines load into it, deleting those Pipelines " +
					"and their Sources too. Destination tables are kept. Without it, destroying a Destination " +
					"that Pipelines still use fails and names them. Defaults to false.",
			},
			"type": schema.StringAttribute{
				Computed:      true,
				Description:   "The destination type, for example `SNOWFLAKE`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "The Destination's status, for example `CREATED`.",
			},
			"created_at": schema.StringAttribute{
				Computed:      true,
				Description:   "When the Destination was created, in RFC 3339.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_by": schema.StringAttribute{
				Computed:      true,
				Description:   "Who created the Destination.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "When the Destination was last changed, in RFC 3339.",
			},
			"updated_by": schema.StringAttribute{
				Computed:    true,
				Description: "Who last changed the Destination.",
			},
		},
	}
}
