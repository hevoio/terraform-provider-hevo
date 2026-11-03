package resources_test

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hevoio/terraform-provider-hevo/hevo/tests/mock"
)

func TestPipelineCreateAdoptsAfterGatewayTimeoutMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	// The create times out at the gateway but finishes in Hevo, after two lists that miss it.
	fake := &fakePipelines{failCreate: http.StatusGatewayTimeout, createdAnyway: true, hiddenLists: 2}
	fake.register(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: pipelineConfig("HubSpot to Snowflake", "60", "pat-1", "MERGE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hevo_pipeline.p", "id", "42"),
				),
			},
		},
	})
}

func TestPipelineCreateFailsWhenNothingWasCreatedMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := &fakePipelines{failCreate: http.StatusGatewayTimeout}
	fake.register(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      pipelineConfig("HubSpot to Snowflake", "60", "pat-1", "MERGE"),
				ExpectError: regexp.MustCompile(`Unable to create Pipeline`),
			},
		},
	})
}

func TestPipelineCreateNeverAdoptsAfterARefusalMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	// A 4xx is a refusal: even with a same-named Pipeline listed, it is not taken over.
	fake := &fakePipelines{failCreate: http.StatusBadRequest, createdAnyway: true}
	fake.register(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      pipelineConfig("HubSpot to Snowflake", "60", "pat-1", "MERGE"),
				ExpectError: regexp.MustCompile(`Unable to create Pipeline`),
			},
		},
	})
}

func TestDestinationCreateAdoptsAfterGatewayTimeoutMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := &fakeDestinations{failCreate: http.StatusGatewayTimeout, createdAnyway: true}
	fake.register(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: destinationConfig("Snowflake", "LOAD_WH", ""),
				Check:  resource.TestCheckResourceAttr("hevo_destination.sf", "id", "7"),
			},
		},
	})
}

func TestDestinationCreateFailsWhenNothingWasCreatedMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := &fakeDestinations{failCreate: http.StatusGatewayTimeout}
	fake.register(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      destinationConfig("Snowflake", "LOAD_WH", ""),
				ExpectError: regexp.MustCompile(`Unable to create Destination`),
			},
		},
	})
}
