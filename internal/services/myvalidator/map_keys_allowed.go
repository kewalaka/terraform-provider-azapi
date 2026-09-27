package myvalidator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type mapKeysAllowed struct {
	allowed []string
}

func (v mapKeysAllowed) Description(ctx context.Context) string {
	return fmt.Sprintf("header names must be one of (case-insensitive): %s", strings.Join(v.allowed, ", "))
}

func (v mapKeysAllowed) MarkdownDescription(ctx context.Context) string {
	return fmt.Sprintf("header names must be one of (case-insensitive): `%s`", strings.Join(v.allowed, "`, `"))
}

func (v mapKeysAllowed) ValidateMap(ctx context.Context, req validator.MapRequest, resp *validator.MapResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	keys := make([]string, 0, len(req.ConfigValue.Elements()))
	for key := range req.ConfigValue.Elements() {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if v.isAllowed(key) {
			continue
		}
		resp.Diagnostics.AddAttributeError(
			req.Path.AtMapKey(key),
			"Header not allowed",
			fmt.Sprintf("header %q is not allowed; allowed headers: %s", key, strings.Join(v.allowed, ", ")),
		)
	}
}

func (v mapKeysAllowed) isAllowed(key string) bool {
	for _, allowed := range v.allowed {
		if strings.EqualFold(key, allowed) {
			return true
		}
	}
	return false
}

// MapKeysAllowed rejects map keys that are not in allowed. Keys are compared case-insensitively
// because it is intended for HTTP header maps, whose names are case-insensitive.
func MapKeysAllowed(allowed ...string) validator.Map {
	sorted := append([]string(nil), allowed...)
	sort.Slice(sorted, func(i, j int) bool {
		return strings.ToLower(sorted[i]) < strings.ToLower(sorted[j])
	})
	return mapKeysAllowed{allowed: sorted}
}
