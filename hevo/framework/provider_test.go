package framework

import (
	"context"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hevoio/hevo-go/tests/mock"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
)

func providerSchema(t *testing.T, p provider.Provider) provider.SchemaResponse {
	t.Helper()

	var resp provider.SchemaResponse

	p.Schema(t.Context(), provider.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema() diagnostics: %v", resp.Diagnostics)
	}

	return resp
}

// configure runs Configure with the given provider block values; unset keys are null.
func configure(t *testing.T, p provider.Provider, values map[string]tftypes.Value) provider.ConfigureResponse {
	t.Helper()

	schemaResp := providerSchema(t, p)

	objectType, ok := schemaResp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
	if !ok {
		t.Fatal("provider schema is not an object")
	}

	attrs := map[string]tftypes.Value{}
	for name, attrType := range objectType.AttributeTypes {
		attrs[name] = tftypes.NewValue(attrType, nil)
	}

	maps.Copy(attrs, values)

	req := provider.ConfigureRequest{
		TerraformVersion: "1.9.8",
		Config: tfsdk.Config{
			Schema: schemaResp.Schema,
			Raw:    tftypes.NewValue(objectType, attrs),
		},
	}

	var resp provider.ConfigureResponse

	p.Configure(t.Context(), req, &resp)

	return resp
}

func providerData(t *testing.T, resp provider.ConfigureResponse) *core.ProviderResourceData {
	t.Helper()

	data, ok := resp.ResourceData.(*core.ProviderResourceData)
	if !ok {
		t.Fatalf("ResourceData = %T, want *core.ProviderResourceData", resp.ResourceData)
	}

	return data
}

func str(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

func TestProviderSchemaIsValid(t *testing.T) {
	t.Parallel()

	resp := providerSchema(t, HevoProvider("test")())
	if diags := resp.Schema.ValidateImplementation(t.Context()); diags.HasError() {
		t.Fatalf("ValidateImplementation() = %v", diags)
	}
}

func TestProviderConfigureRequiresKeys(t *testing.T) {
	t.Setenv(EnvAccessKey, "")
	t.Setenv(EnvSecretKey, "")

	resp := configure(t, HevoProvider("test")(), nil)

	if resp.Diagnostics.ErrorsCount() != 2 {
		t.Fatalf("diagnostics = %v, want missing access key and secret key", resp.Diagnostics)
	}
}

func TestProviderConfigureReadsEnvironment(t *testing.T) {
	t.Setenv(EnvAccessKey, "env-key")
	t.Setenv(EnvSecretKey, "env-secret")
	t.Setenv(EnvRegion, "eu")
	t.Setenv(EnvHost, "")

	resp := configure(t, HevoProvider("test")(), nil)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	data := providerData(t, resp)
	if got := data.Client.HTTPService().BaseURL; got != "https://eu.hevodata.com" {
		t.Errorf("BaseURL = %q, want the eu host", got)
	}
}

func TestProviderConfigureHostWinsOverEnvironmentRegion(t *testing.T) {
	t.Setenv(EnvRegion, "eu")

	resp := configure(t, HevoProvider("test")(), map[string]tftypes.Value{
		"access_key": str("key"),
		"secret_key": str("secret"),
		"host":       str("http://localhost:9080/"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	data := providerData(t, resp)
	if got := data.Client.HTTPService().BaseURL; got != "http://localhost:9080" {
		t.Errorf("BaseURL = %q, want the host", got)
	}
}

func TestProviderConfigureRejectsUnknownSettings(t *testing.T) {
	t.Parallel()

	resp := configure(t, HevoProvider("test")(), map[string]tftypes.Value{
		"access_key": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"secret_key": str("secret"),
	})
	if !resp.Diagnostics.HasError() {
		t.Fatal("want an error for an access key unknown until apply")
	}
}

func TestProviderConfigureSetsUserAgentAndSkipValidation(t *testing.T) {
	t.Parallel()

	mockClient := mock.NewHTTPClient()
	mockClient.When(http.MethodGet, "/api/v2/pipelines/1").ThenCall(func(req *http.Request) (*http.Response, error) {
		ua := req.Header.Get("User-Agent")
		if !strings.HasSuffix(ua, "terraform-provider-hevo/test terraform/1.9.8") {
			t.Errorf("User-Agent = %q", ua)
		}

		return mock.NewResponse(req, http.StatusOK, `{"code":"Success","data":{"id":1}}`), nil
	})

	resp := configure(t, HevoProviderMock(mockClient), map[string]tftypes.Value{
		"access_key":                str("key"),
		"secret_key":                str("secret"),
		"skip_plan_time_validation": tftypes.NewValue(tftypes.Bool, true),
	})
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}

	data := providerData(t, resp)
	if !data.SkipPlanTimeValidation {
		t.Error("SkipPlanTimeValidation = false, want true")
	}

	if _, err := data.Client.GetPipeline().PipelineID(1).Do(t.Context()); err != nil {
		t.Fatal(err)
	}
}
