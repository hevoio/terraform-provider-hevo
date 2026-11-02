package core

import (
	"context"
	"errors"
	"net/http"
	"testing"

	httputils "github.com/hevoio/hevo-go/http_utils"
)

func TestMayHaveBeenCreated(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err  error
		want bool
	}{
		"no error":        {nil, false},
		"network error":   {errors.New("connection reset by peer"), true},
		"timeout":         {context.DeadlineExceeded, true},
		"gateway timeout": {&httputils.ResponseError{StatusCode: http.StatusGatewayTimeout}, true},
		"server error":    {&httputils.ResponseError{StatusCode: http.StatusInternalServerError}, true},
		"bad request":     {&httputils.ResponseError{StatusCode: http.StatusBadRequest}, false},
		"rate limited":    {&httputils.ResponseError{StatusCode: http.StatusTooManyRequests}, false},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := MayHaveBeenCreated(tt.err); got != tt.want {
				t.Errorf("MayHaveBeenCreated(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
