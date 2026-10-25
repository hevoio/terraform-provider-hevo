package core

import (
	"sync"

	"github.com/hevoio/hevo-go"
)

// ProviderResourceData is what the provider hands every resource and data source.
type ProviderResourceData struct {
	Client *hevo.Client
	// MetadataCache keeps each connector's schema for one run, so plan-time validation fetches it
	// once per connector.
	MetadataCache          *sync.Map
	SkipPlanTimeValidation bool
}
