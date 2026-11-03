package core

import (
	"errors"
	"net/http"
	"strings"

	httputils "github.com/hevoio/hevo-go/http_utils"
)

// SchemaReadErrorDetail is the text of a failure to read a connector's schema at plan time. It
// suggests planning without the schema, except for a 401, which fails every call.
func SchemaReadErrorDetail(err error) string {
	if isUnauthorized(err) {
		return ErrorDetail(err)
	}

	return strings.TrimSuffix(err.Error(), ".") + ". Set skip_plan_time_validation = true in the provider block " +
		"to plan without it; config mistakes then surface at apply."
}

func isUnauthorized(err error) bool {
	respErr, ok := errors.AsType[*httputils.ResponseError](err)

	return ok && respErr.StatusCode == http.StatusUnauthorized
}

// ErrorDetail is the text of an error for a diagnostic: the API's own message. A 401 also says
// which host the keys were sent to, since keys of a team on another Hevo host get the same answer
// as wrong keys.
func ErrorDetail(err error) string {
	if isUnauthorized(err) {
		return err.Error() + "\n\nCheck that access_key and secret_key belong to a team on the host the provider " +
			"uses: host in the provider block, else the " + "HEVO_HOST environment variable, else https://us.hevodata.com."
	}

	return err.Error()
}
