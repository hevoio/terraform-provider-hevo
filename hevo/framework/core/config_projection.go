package core

import (
	"context"
	"fmt"
	"math/big"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DynamicToMap converts a dynamic attribute holding an object into a map. It returns nil for a
// null or unknown value. Numbers become int64 when whole, else float64.
func DynamicToMap(dyn types.Dynamic) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	if dyn.IsNull() || dyn.IsUnknown() || dyn.IsUnderlyingValueNull() || dyn.IsUnderlyingValueUnknown() {
		return nil, diags
	}

	m, ok := attrToInterface(dyn.UnderlyingValue()).(map[string]any)
	if !ok {
		diags.AddError("Config is not an object", "Set config to an object, for example config = { key = \"value\" }.")

		return nil, diags
	}

	return m, diags
}

// IsFullyKnown reports whether a dynamic value holds no unknown values at any depth.
func IsFullyKnown(dyn types.Dynamic) bool {
	return !dyn.IsUnknown() && (dyn.IsNull() || isKnown(dyn.UnderlyingValue()))
}

func isKnown(v attr.Value) bool {
	if v == nil || v.IsNull() {
		return true
	}

	if v.IsUnknown() {
		return false
	}

	switch val := v.(type) {
	case types.Object:
		return allKnown(val.Attributes())
	case types.Map:
		return allKnown(val.Elements())
	case types.Tuple:
		return !slices.ContainsFunc(val.Elements(), func(e attr.Value) bool { return !isKnown(e) })
	case types.List:
		return !slices.ContainsFunc(val.Elements(), func(e attr.Value) bool { return !isKnown(e) })
	case types.Set:
		return !slices.ContainsFunc(val.Elements(), func(e attr.Value) bool { return !isKnown(e) })
	case types.Dynamic:
		return IsFullyKnown(val)
	default:
		return true
	}
}

func allKnown(values map[string]attr.Value) bool {
	for _, v := range values {
		if !isKnown(v) {
			return false
		}
	}

	return true
}

func attrToInterface(v attr.Value) any {
	if v == nil || v.IsNull() || v.IsUnknown() {
		return nil
	}

	if scalar, ok := scalarToInterface(v); ok {
		return scalar
	}

	switch val := v.(type) {
	case types.Object:
		return objectToMap(val.Attributes())
	case types.Map:
		return objectToMap(val.Elements())
	case types.Tuple:
		return sliceToInterfaces(val.Elements())
	case types.List:
		return sliceToInterfaces(val.Elements())
	case types.Set:
		return sliceToInterfaces(val.Elements())
	case types.Dynamic:
		return attrToInterface(val.UnderlyingValue())
	default:
		return nil
	}
}

func scalarToInterface(v attr.Value) (any, bool) {
	switch val := v.(type) {
	case types.String:
		return val.ValueString(), true
	case types.Bool:
		return val.ValueBool(), true
	case types.Int64:
		return val.ValueInt64(), true
	case types.Float64:
		return val.ValueFloat64(), true
	case types.Number:
		return bigFloatToInterface(val.ValueBigFloat()), true
	default:
		return nil, false
	}
}

func bigFloatToInterface(f *big.Float) any {
	if f.IsInt() {
		if i, accuracy := f.Int64(); accuracy == big.Exact {
			return i
		}
	}

	value, _ := f.Float64()

	return value
}

func objectToMap(values map[string]attr.Value) map[string]any {
	m := make(map[string]any, len(values))
	for k, v := range values {
		m[k] = attrToInterface(v)
	}

	return m
}

func sliceToInterfaces(values []attr.Value) []any {
	s := make([]any, 0, len(values))
	for _, v := range values {
		s = append(s, attrToInterface(v))
	}

	return s
}

// MapToDynamic converts a map back into a dynamic object value.
func MapToDynamic(m map[string]any) (types.Dynamic, diag.Diagnostics) {
	if m == nil {
		return types.DynamicNull(), nil
	}

	value, diags := interfaceToAttr(m)

	return types.DynamicValue(value), diags
}

func interfaceToAttr(v any) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	switch val := v.(type) {
	case nil:
		return types.StringNull(), diags
	case string:
		return types.StringValue(val), diags
	case bool:
		return types.BoolValue(val), diags
	case int:
		return types.NumberValue(big.NewFloat(float64(val))), diags
	case int64:
		return types.NumberValue(new(big.Float).SetInt64(val)), diags
	case float64:
		return types.NumberValue(big.NewFloat(val)), diags
	case map[string]any:
		attrTypes := make(map[string]attr.Type, len(val))
		attrValues := make(map[string]attr.Value, len(val))

		for k, item := range val {
			converted, d := interfaceToAttr(item)
			diags.Append(d...)

			attrTypes[k] = converted.Type(context.Background())
			attrValues[k] = converted
		}

		object, d := types.ObjectValue(attrTypes, attrValues)
		diags.Append(d...)

		return object, diags
	case []any:
		elemTypes := make([]attr.Type, 0, len(val))
		elems := make([]attr.Value, 0, len(val))

		for _, item := range val {
			converted, d := interfaceToAttr(item)
			diags.Append(d...)

			elemTypes = append(elemTypes, converted.Type(context.Background()))
			elems = append(elems, converted)
		}

		tuple, d := types.TupleValue(elemTypes, elems)
		diags.Append(d...)

		return tuple, diags
	default:
		diags.AddError("Unsupported config value", fmt.Sprintf("Config value %v has unsupported type %T.", v, v))

		return types.StringNull(), diags
	}
}

// ProjectConfig narrows a config read back from the API onto the keys the user manages (mask),
// so keys Hevo adds do not show as drift. It follows these rules per key:
//
//   - writeOnly (or a masked value when the schema is unavailable): keep the local value, since
//     the API never returns it.
//   - in the mask but absent remotely: null, to surface the drift.
//   - an object: recurse.
//   - a scalar equal to the local value except in type (443 and "443"): keep the local value.
//   - otherwise: take the remote value.
//
// A nil mask means an import: every remote key is kept except credentials, which are unknown.
func ProjectConfig(remote, mask map[string]any, schema *Property) map[string]any {
	if mask == nil {
		return importConfig(remote, schema)
	}

	result := make(map[string]any, len(mask))

	for key, local := range mask {
		prop := schema.property(key)
		remoteValue, present := remote[key]

		switch {
		case prop != nil && prop.WriteOnly, isMasked(remoteValue):
			result[key] = local
		case !present:
			result[key] = nil
		default:
			result[key] = projectValue(remoteValue, local, prop)
		}
	}

	return result
}

func projectValue(remote, local any, prop *Property) any {
	remoteMap, remoteIsMap := remote.(map[string]any)
	localMap, localIsMap := local.(map[string]any)

	if remoteIsMap && localIsMap {
		return ProjectConfig(remoteMap, localMap, prop)
	}

	if ScalarsEqual(remote, local) {
		return local
	}

	return remote
}

func importConfig(remote map[string]any, schema *Property) map[string]any {
	result := make(map[string]any, len(remote))

	for key, value := range remote {
		prop := schema.property(key)
		if (prop != nil && (prop.WriteOnly || prop.ReadOnly)) || isMasked(value) {
			continue
		}

		if nested, ok := value.(map[string]any); ok {
			value = importConfig(nested, prop)
		}

		result[key] = value
	}

	return result
}

// isMasked reports whether an API value is a masked credential: a non-empty run of asterisks.
func isMasked(v any) bool {
	s, ok := v.(string)

	return ok && s != "" && strings.Trim(s, "*") == ""
}

// ScalarsEqual reports whether two values are equal, treating a number and its string form as
// equal: Hevo may read back 443 for a port written as "443".
func ScalarsEqual(a, b any) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}

	as, aok := scalarString(a)
	bs, bok := scalarString(b)

	return aok && bok && as == bs
}

func scalarString(v any) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case bool:
		return strconv.FormatBool(val), true
	case int64:
		return strconv.FormatInt(val, 10), true
	case int:
		return strconv.Itoa(val), true
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64), true
	default:
		return "", false
	}
}

// ChangedConfig returns the keys of plan whose values differ from state: the partial body an
// update sends. Keys removed from plan are left out, so Hevo keeps their current value.
func ChangedConfig(plan, state map[string]any) map[string]any {
	changed := map[string]any{}

	for key, value := range plan {
		if old, ok := state[key]; ok && reflect.DeepEqual(old, value) {
			continue
		}

		changed[key] = value
	}

	return changed
}
