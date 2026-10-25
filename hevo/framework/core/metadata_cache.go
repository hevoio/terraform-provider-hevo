package core

import (
	"context"
	"fmt"
	"sync"

	"github.com/hevoio/hevo-go"
)

// GetSourceConnectorSchema returns the config schema of a source connector, fetching it once per
// provider run.
func GetSourceConnectorSchema(
	ctx context.Context,
	client *hevo.Client,
	cache *sync.Map,
	connectorID string,
) (*Property, error) {
	return cachedSchema(ctx, cache, "source/"+connectorID, func() (map[string]any, error) {
		resp, err := client.GetSourceConnector().ConnectorID(connectorID).Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading the schema of source connector %q: %w", connectorID, err)
		}

		return resp.Data.Schema, nil
	})
}

// GetDestinationConnectorSchema returns the config schema of a destination connector, fetching
// it once per provider run. Errors are not cached, so a transient failure is retried next call.
func GetDestinationConnectorSchema(
	ctx context.Context,
	client *hevo.Client,
	cache *sync.Map,
	connectorID string,
) (*Property, error) {
	return cachedSchema(ctx, cache, "destination/"+connectorID, func() (map[string]any, error) {
		resp, err := client.GetDestinationConnector().ConnectorID(connectorID).Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading the schema of destination connector %q: %w", connectorID, err)
		}

		return resp.Data.Schema, nil
	})
}

// cachedSchema returns the parsed config schema under key, fetching it on a miss. Errors are not
// cached, so a transient failure is retried on the next call.
func cachedSchema(_ context.Context, cache *sync.Map, key string, fetch func() (map[string]any, error)) (*Property, error) {
	if cached, ok := cache.Load(key); ok {
		if p, ok := cached.(*Property); ok {
			return p, nil
		}
	}

	schema, err := fetch()
	if err != nil {
		return nil, err
	}

	p := ParseConnectorConfigSchema(schema)
	cache.Store(key, p)

	return p, nil
}
