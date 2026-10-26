package datasources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hevoio/hevo-go"
	"github.com/hevoio/hevo-go/metadata"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core/model"
	hevoschema "github.com/hevoio/terraform-provider-hevo/hevo/framework/core/schema"
)

var pathType = path.Root("type")

// ConnectorMetadata is the hevo_connector_metadata data source.
func ConnectorMetadata() datasource.DataSource {
	return &connectorMetadata{}
}

type connectorMetadata struct {
	core.ProviderDatasource
}

var _ datasource.DataSourceWithConfigure = (*connectorMetadata)(nil)

func (d *connectorMetadata) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connector_metadata"
}

func (d *connectorMetadata) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = hevoschema.ConnectorMetadataDatasourceSchema()
}

func (d *connectorMetadata) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.GetClient() == nil {
		core.AddUnconfiguredClientError(&resp.Diagnostics)

		return
	}

	var data model.ConnectorMetadataModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var (
		details metadata.ConnectorDetailsResponse
		err     error
	)

	connectorID := data.ConnectorID.ValueString()
	if data.Kind.ValueString() == hevoschema.ConnectorKindDestination {
		details, err = d.GetClient().GetDestinationConnector().ConnectorID(connectorID).Do(ctx)
	} else {
		details, err = d.GetClient().GetSourceConnector().ConnectorID(connectorID).Do(ctx)
	}

	if hevo.IsNotFound(err) {
		resp.Diagnostics.AddAttributeError(path.Root("connector_id"), "Unknown connector",
			"No "+data.Kind.ValueString()+" connector has id "+connectorID+". List them with the hevo_connectors data source.")

		return
	}

	if err != nil {
		resp.Diagnostics.AddError("Unable to read connector", err.Error())

		return
	}

	resp.Diagnostics.Append(data.ReadFromResponse(ctx, details.Data)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
