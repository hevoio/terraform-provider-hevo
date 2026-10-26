package core

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/hevoio/hevo-go"
	"github.com/hevoio/hevo-go/pipelines"
)

// Pipeline states the API reports while an action or edit is being applied.
var transitionalPipelineStates = []string{"Initializing", "Enabling", "Disabling", "Restarting", "Deleting"}

const (
	firstPollInterval = 2 * time.Second
	maxPollInterval   = 30 * time.Second
)

// PollInterval is the first wait between polls. Unit tests shorten it.
var PollInterval = firstPollInterval

// IsTransitionalPipelineState reports whether a Pipeline is still applying an action or edit.
func IsTransitionalPipelineState(state string) bool {
	return slices.Contains(transitionalPipelineStates, state)
}

// WaitForPipeline reads the Pipeline until it leaves a transitional state, backing off from 2 s
// to 30 s. Bound it with a deadline on ctx, from the resource's timeouts.
func WaitForPipeline(ctx context.Context, client *hevo.Client, pipelineID int64) (pipelines.PipelineDetails, error) {
	wait := PollInterval

	for {
		resp, err := client.GetPipeline().PipelineID(pipelineID).Do(ctx)
		if err != nil {
			return pipelines.PipelineDetails{}, err
		}

		if !IsTransitionalPipelineState(resp.Data.State) {
			if resp.Data.State == "Failed" {
				return resp.Data, fmt.Errorf("pipeline %d is in state Failed", pipelineID)
			}

			return resp.Data, nil
		}

		if err := sleep(ctx, wait); err != nil {
			return resp.Data, fmt.Errorf("pipeline %d still %s: %w", pipelineID, resp.Data.State, err)
		}

		wait = min(wait*2, maxPollInterval)
	}
}

// WaitUntil calls check until it reports done or fails, backing off like WaitForPipeline. Bound it
// with a deadline on ctx.
func WaitUntil(ctx context.Context, what string, check func(context.Context) (bool, error)) error {
	wait := PollInterval

	for {
		done, err := check(ctx)
		if err != nil || done {
			return err
		}

		if err := sleep(ctx, wait); err != nil {
			return fmt.Errorf("gave up waiting for %s: %w", what, err)
		}

		wait = min(wait*2, maxPollInterval)
	}
}

// RetryOnConflict calls fn until it succeeds or fails with something other than a 409, backing
// off like WaitForPipeline. A 409 means the Pipeline is mid-transition or has an edit open in the
// Hevo UI, and the call was not applied, so repeating it is safe.
func RetryOnConflict(ctx context.Context, fn func(context.Context) error) error {
	wait := PollInterval

	for {
		err := fn(ctx)
		if !hevo.IsConflict(err) {
			return err
		}

		if waitErr := sleep(ctx, wait); waitErr != nil {
			return fmt.Errorf("%w (gave up waiting: %w)", err, waitErr)
		}

		wait = min(wait*2, maxPollInterval)
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	d += rand.N(d/5 + 1) //nolint:gosec // jitter needs no cryptographic randomness

	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
