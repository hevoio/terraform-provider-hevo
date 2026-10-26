// Package mock runs the provider in Terraform against an in-memory API, for resource and data
// source unit tests:
//
//	client, factories := mock.NewProvider()
//	client.When(http.MethodGet, "/api/v2/destinations/7").ThenReply(200, body)
//	resource.UnitTest(t, resource.TestCase{ProtoV6ProviderFactories: factories, Steps: ...})
package mock

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	sdkmock "github.com/hevoio/hevo-go/tests/mock"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework"
)

// ProviderConfig is the provider block every mock test uses.
const ProviderConfig = `
provider "hevo" {
  access_key = "test_access_key"
  secret_key = "test_secret_key"
  host       = "https://mock.hevodata.test"
}
`

// NewProvider returns a fresh in-memory API and provider factories that send requests to it.
func NewProvider() (*sdkmock.HTTPClient, map[string]func() (tfprotov6.ProviderServer, error)) {
	client := sdkmock.NewHTTPClient()

	return client, map[string]func() (tfprotov6.ProviderServer, error){
		"hevo": providerserver.NewProtocol6WithError(framework.HevoProviderMock(client)),
	}
}
