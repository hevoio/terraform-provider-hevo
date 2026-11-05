package core

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/hevoio/hevo-go-sdk"
	"github.com/hevoio/hevo-go-sdk/common"
	"github.com/hevoio/hevo-go-sdk/destinations"
	httputils "github.com/hevoio/hevo-go-sdk/http_utils"
	"github.com/hevoio/hevo-go-sdk/pipelines"
)

// CreateRecoveryWait bounds the search for a resource that a failed create may still have made.
// Unit tests shorten it.
var CreateRecoveryWait = 5 * time.Minute

const recoveryPageLimit = 500

// MayHaveBeenCreated reports whether a failed create could still have created the resource: no
// response arrived (a timeout or a broken connection), or the gateway or server failed. A 4xx means
// the request was refused.
func MayHaveBeenCreated(err error) bool {
	if respErr, ok := errors.AsType[*httputils.ResponseError](err); ok {
		return respErr.StatusCode >= http.StatusInternalServerError
	}

	return err != nil
}

// RecoveryContext returns the context to look for, and wait on, a resource that a failed create may
// have made. When the create ran out of time, the search gets its own CreateRecoveryWait, so the
// resource is not left out of state; an interrupt still stops it.
func RecoveryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		ctx = context.WithoutCancel(ctx)
	}

	return context.WithTimeout(ctx, CreateRecoveryWait)
}

// FindCreatedPipeline looks a Pipeline up by its name, which is unique in a team, until it appears
// or ctx ends: after a gateway timeout the create can still be finishing.
//
//nolint:dupl // the same lookup over a different list call and type
func FindCreatedPipeline(ctx context.Context, client *hevo.Client, name string) (pipelines.PipelineDetails, bool) {
	return findCreated(ctx, name, func(ctx context.Context, cursor *string) ([]pipelines.PipelineDetails, common.ListResponse, error) {
		call := client.ListPipelines().Limit(recoveryPageLimit)
		if cursor != nil {
			call.Cursor(*cursor)
		}

		page, err := call.Do(ctx)

		return page.Data, page.ListResponse, err
	}, func(p pipelines.PipelineDetails) string { return p.Name })
}

// FindCreatedDestination looks a Destination up by its name, which is unique in a team, until it
// appears or ctx ends.
//
//nolint:dupl // the same lookup over a different list call and type
func FindCreatedDestination(ctx context.Context, client *hevo.Client, name string) (destinations.DestinationDetails, bool) {
	return findCreated(ctx, name, func(ctx context.Context, cursor *string) ([]destinations.DestinationDetails, common.ListResponse, error) {
		call := client.ListDestinations().Limit(recoveryPageLimit)
		if cursor != nil {
			call.Cursor(*cursor)
		}

		page, err := call.Do(ctx)

		return page.Data, page.ListResponse, err
	}, func(d destinations.DestinationDetails) string { return d.Name })
}

// findCreated pages through list for the item called name, polling again until ctx ends.
func findCreated[T any](
	ctx context.Context,
	name string,
	list func(context.Context, *string) ([]T, common.ListResponse, error),
	nameOf func(T) string,
) (T, bool) {
	var found T

	ok := false

	_ = WaitUntil(ctx, "the created resource", func(ctx context.Context) (bool, error) {
		var cursor *string

		for {
			items, page, err := list(ctx, cursor)
			if err != nil {
				// A failed list counts as not found yet; the next poll tries again.
				return false, nil //nolint:nilerr // see above
			}

			for _, item := range items {
				if nameOf(item) == name {
					found, ok = item, true

					return true, nil
				}
			}

			if !page.HasMore || page.NextCursor == "" {
				return false, nil
			}

			cursor = &page.NextCursor
		}
	})

	return found, ok
}
