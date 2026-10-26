package resources

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hevoio/hevo-go"
	"github.com/hevoio/hevo-go/pipelines"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core/model"
)

func schedule(m *model.ScheduleModel) *pipelines.Schedule {
	s := hevo.NewSchedule().SyncType(m.SyncType.ValueString())
	if !m.FrequencyMinutes.IsNull() {
		s.FrequencyMinutes(m.FrequencyMinutes.ValueInt64())
	}

	return s
}

func failureHandlingPolicy(m *model.FailureHandlingPolicyModel) *pipelines.FailureHandlingPolicy {
	p := hevo.NewFailureHandlingPolicy().ObjectFailureLevel(m.ObjectFailureLevel.ValueString())
	if !m.ObjectFailureThreshold.IsNull() {
		p.ObjectFailureThreshold(m.ObjectFailureThreshold.ValueInt64())
	}

	return p
}

func latencyAlert(m *model.LatencyAlertModel) *pipelines.LatencyAlert {
	a := hevo.NewLatencyAlert().Enabled(m.Enabled.ValueBool())
	if !m.ThresholdMinutes.IsNull() {
		a.ThresholdMinutes(int(m.ThresholdMinutes.ValueInt64()))
	}

	return a
}

func equalSchedule(a, b *model.ScheduleModel) bool {
	return a != nil && b != nil && a.SyncType.Equal(b.SyncType) && a.FrequencyMinutes.Equal(b.FrequencyMinutes)
}

func equalFailurePolicy(a, b *model.FailureHandlingPolicyModel) bool {
	return a != nil && b != nil && a.ObjectFailureLevel.Equal(b.ObjectFailureLevel) &&
		a.ObjectFailureThreshold.Equal(b.ObjectFailureThreshold)
}

func equalLatencyAlert(a, b *model.LatencyAlertModel) bool {
	return a != nil && b != nil && a.Enabled.Equal(b.Enabled) && a.ThresholdMinutes.Equal(b.ThresholdMinutes)
}

// sourceAuth converts source_auth into the SDK's builder; nil when source_auth is null.
func sourceAuth(value types.Dynamic) (*pipelines.SourceAuth, diag.Diagnostics) {
	m, diags := core.DynamicToMap(value)
	if m == nil || diags.HasError() {
		return nil, diags
	}

	auth := hevo.NewSourceAuth()

	for key, v := range m {
		switch key {
		case "client_access":
			access, ok := v.(map[string]any)
			if !ok {
				diags.AddAttributeError(path.Root("source_auth").AtMapKey(key), "Invalid source_auth",
					"client_access must be an object, for example { client_id = \"...\", client_secret = \"...\" }.")

				continue
			}

			for name, credential := range access {
				auth.ClientAccessValue(name, fmt.Sprint(credential))
			}
		case "refresh_token":
			auth.RefreshToken(fmt.Sprint(v))
		case "access_token":
			auth.AccessToken(fmt.Sprint(v))
		default:
			diags.AddAttributeError(path.Root("source_auth").AtMapKey(key), "Unknown source_auth key",
				"source_auth takes client_access, refresh_token and access_token.")
		}
	}

	return auth, diags
}
