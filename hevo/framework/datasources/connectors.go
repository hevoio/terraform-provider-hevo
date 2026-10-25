// Package datasources holds the provider's data sources, one file each.
package datasources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hevoio/hevo-go/metadata"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core/model"
	hevoschema "github.com/hevoio/terraform-provider-hevo/hevo/framework/core/schema"
)

// pageLimit is the largest page the API serves.
const pageLimit = 500

// Connectors is the hevo_connectors data source.
func Connectors() datasource.DataSource {
	return &connectors{}
}

type connectors struct {
	core.ProviderDatasource
}

var _ datasource.DataSourceWithConfigure = (*connectors)(nil)

func (d *connectors) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connectors"
}

func (d *connectors) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = hevoschema.ConnectorsDatasourceSchema()
}

func (d *connectors) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.GetClient() == nil {
		core.AddUnconfiguredClientError(&resp.Diagnostics)

		return
	}

	var data model.ConnectorsModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Kind.ValueString() == hevoschema.ConnectorKindDestination && data.Type.ValueString() != "" {
		resp.Diagnostics.AddAttributeError(pathType, "Filter not supported",
			"type filters source connectors only; remove it for kind = \"destination\".")

		return
	}

	all := []metadata.Connector{}
	cursor := ""

	for {
		page, err := d.listPage(ctx, data, cursor)
		if err != nil {
			resp.Diagnostics.AddError("Unable to list connectors", err.Error())

			return
		}

		all = append(all, page.Data...)

		if !page.HasMore || page.NextCursor == "" {
			break
		}

		cursor = page.NextCursor
	}

	data.ReadFromResponse(all)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *connectors) listPage(ctx context.Context, data model.ConnectorsModel, cursor string) (metadata.ConnectorsListResponse, error) {
	if data.Kind.ValueString() == hevoschema.ConnectorKindDestination {
		call := d.GetClient().ListDestinationConnectors().Limit(pageLimit)
		if cursor != "" {
			call.Cursor(cursor)
		}

		return call.Do(ctx)
	}

	call := d.GetClient().ListSourceConnectors().Limit(pageLimit)
	if cursor != "" {
		call.Cursor(cursor)
	}

	if t := data.Type.ValueString(); t != "" {
		call.Type(t)
	}

	return call.Do(ctx)
}
