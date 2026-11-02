package core

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	httputils "github.com/hevoio/hevo-go/http_utils"
)

func TestMain(m *testing.M) {
	PollInterval, ConflictRetryWait = time.Millisecond, 50*time.Millisecond

	os.Exit(m.Run())
}

func TestRetryOnConflict(t *testing.T) {
	t.Parallel()

	conflict := &httputils.ResponseError{StatusCode: http.StatusConflict, Message: "Edit in progress."}

	t.Run("retries until the 409 clears", func(t *testing.T) {
		t.Parallel()

		calls := 0

		err := RetryOnConflict(context.Background(), func(context.Context) error {
			calls++
			if calls < 3 {
				return conflict
			}

			return nil
		})
		if err != nil || calls != 3 {
			t.Errorf("err = %v after %d calls, want success on the third", err, calls)
		}
	})

	t.Run("returns the API's 409 when it never clears", func(t *testing.T) {
		t.Parallel()

		err := RetryOnConflict(context.Background(), func(context.Context) error { return conflict })
		if !errors.Is(err, conflict) {
			t.Errorf("err = %v, want the API's 409", err)
		}
	})

	t.Run("does not retry other errors", func(t *testing.T) {
		t.Parallel()

		calls := 0
		badRequest := &httputils.ResponseError{StatusCode: http.StatusBadRequest}

		err := RetryOnConflict(context.Background(), func(context.Context) error {
			calls++

			return badRequest
		})
		if !errors.Is(err, badRequest) || calls != 1 {
			t.Errorf("err = %v after %d calls, want the 400 after one call", err, calls)
		}
	})
}
