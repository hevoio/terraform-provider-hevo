package core

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	httputils "github.com/hevoio/hevo-go-sdk/http_utils"
)

func TestErrorDetail(t *testing.T) {
	t.Parallel()

	unauthorized := &httputils.ResponseError{StatusCode: http.StatusUnauthorized}
	if got := ErrorDetail(unauthorized); !strings.HasPrefix(got, "hevo: 401: Unauthorized\n\n") || !strings.Contains(got, "HEVO_HOST") {
		t.Errorf("ErrorDetail(401) = %q, want the API's error and the host hint", got)
	}

	for _, err := range []error{
		&httputils.ResponseError{StatusCode: http.StatusBadRequest, Code: "INVALID_INPUT", Message: "Bad."},
		errors.New("context deadline exceeded"),
	} {
		if got := ErrorDetail(err); got != err.Error() {
			t.Errorf("ErrorDetail(%v) = %q, want the error as is", err, got)
		}
	}
}

func TestSchemaReadErrorDetail(t *testing.T) {
	t.Parallel()

	notFound := &httputils.ResponseError{StatusCode: http.StatusNotFound, Code: "RESOURCE_NOT_FOUND", Message: "No connector `x`."}
	if got := SchemaReadErrorDetail(notFound); !strings.Contains(got, "`x`. Set skip_plan_time_validation") {
		t.Errorf("SchemaReadErrorDetail(404) = %q, want one period before the hint", got)
	}

	if got := SchemaReadErrorDetail(&httputils.ResponseError{StatusCode: http.StatusUnauthorized}); strings.Contains(got, "skip_plan_time_validation") {
		t.Errorf("SchemaReadErrorDetail(401) = %q, want no hint to skip validation", got)
	}
}
