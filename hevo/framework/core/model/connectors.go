package model

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hevoio/hevo-go-sdk/metadata"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
)

// ConnectorsModel is the Terraform value of the hevo_connectors data source.
type ConnectorsModel struct {
	Kind       types.String     `tfsdk:"kind"`
	Type       types.String     `tfsdk:"type"`
	Connectors []ConnectorModel `tfsdk:"connectors"`
}

// ConnectorModel is one connector in the list.
type ConnectorModel struct {
	ConnectorID      types.String `tfsdk:"connector_id"`
	Type             types.String `tfsdk:"type"`
	DisplayName      types.String `tfsdk:"display_name"`
	Description      types.String `tfsdk:"description"`
	DocumentationURL types.String `tfsdk:"documentation_url"`
	InBeta           types.Bool   `tfsdk:"in_beta"`
}

// ReadFromResponse sets the list from every page of connectors.
func (m *ConnectorsModel) ReadFromResponse(connectors []metadata.Connector) {
	m.Connectors = make([]ConnectorModel, 0, len(connectors))
	for _, c := range connectors {
		m.Connectors = append(m.Connectors, ConnectorModel{
			ConnectorID:      types.StringValue(c.ConnectorID),
			Type:             types.StringValue(c.Type),
			DisplayName:      types.StringValue(c.DisplayName),
			Description:      types.StringPointerValue(c.Description),
			DocumentationURL: types.StringPointerValue(c.DocumentationURL),
			InBeta:           types.BoolValue(c.InBeta),
		})
	}
}

// ConnectorMetadataModel is the Terraform value of the hevo_connector_metadata data source.
type ConnectorMetadataModel struct {
	Kind                types.String `tfsdk:"kind"`
	ConnectorID         types.String `tfsdk:"connector_id"`
	Type                types.String `tfsdk:"type"`
	DisplayName         types.String `tfsdk:"display_name"`
	SourceTypes         types.List   `tfsdk:"source_types"`
	SchemaJSON          types.String `tfsdk:"schema_json"`
	RequiredConfigKeys  types.List   `tfsdk:"required_config_keys"`
	WriteOnlyConfigKeys types.List   `tfsdk:"write_only_config_keys"`
}

// ReadFromResponse sets the connector, its schema and the keys derived from it.
func (m *ConnectorMetadataModel) ReadFromResponse(ctx context.Context, d metadata.ConnectorDetails) diag.Diagnostics {
	var diags diag.Diagnostics

	schemaJSON, err := json.Marshal(d.Schema)
	if err != nil {
		diags.AddError("Unable to encode the connector schema", fmt.Sprint(err))

		return diags
	}

	// A source connector names its variants, and its default, in the schema's type property.
	variants, defaultType := sourceTypes(d.Schema)
	if d.Type != "" {
		defaultType = d.Type
	}

	m.ConnectorID = types.StringValue(d.ConnectorID)
	m.Type = types.StringValue(defaultType)
	m.DisplayName = types.StringValue(d.DisplayName)
	m.SchemaJSON = types.StringValue(string(schemaJSON))

	config := core.ParseConnectorConfigSchema(d.Schema)

	var required, writeOnly []string

	if config != nil {
		required = append(required, config.Required...)

		for name, prop := range config.Properties {
			if prop.WriteOnly {
				writeOnly = append(writeOnly, name)
			}
		}
	}

	sort.Strings(writeOnly)

	var d2 diag.Diagnostics

	m.RequiredConfigKeys, d2 = types.ListValueFrom(ctx, types.StringType, nonNil(required))
	diags.Append(d2...)
	m.WriteOnlyConfigKeys, d2 = types.ListValueFrom(ctx, types.StringType, nonNil(writeOnly))
	diags.Append(d2...)
	m.SourceTypes, d2 = types.ListValueFrom(ctx, types.StringType, nonNil(variants))
	diags.Append(d2...)

	return diags
}

// sourceTypes returns the variants a source connector's schema lists under properties.type, and
// its default. Destination schemas have none.
func sourceTypes(schema map[string]any) ([]string, string) {
	properties, _ := schema["properties"].(map[string]any)
	typeProperty, _ := properties["type"].(map[string]any)
	enum, _ := typeProperty["enum"].([]any)
	defaultType, _ := typeProperty["default"].(string)

	variants := make([]string, 0, len(enum))

	for _, v := range enum {
		if s, ok := v.(string); ok {
			variants = append(variants, s)
		}
	}

	return variants, defaultType
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}

	return s
}
