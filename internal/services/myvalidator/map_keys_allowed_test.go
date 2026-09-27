package myvalidator

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestMapKeysAllowed_ValidateMap(t *testing.T) {
	v := MapKeysAllowed("x-ms-version", "Accept", "DataServiceVersion", "MaxDataServiceVersion", "x-ms-client-request-id")
	headers := func(keys ...string) types.Map {
		elements := make(map[string]attr.Value, len(keys))
		for _, key := range keys {
			elements[key] = types.StringValue("value")
		}
		return types.MapValueMust(types.StringType, elements)
	}

	testCases := []struct {
		name           string
		value          types.Map
		expectedErrors []string
	}{
		{name: "null map", value: types.MapNull(types.StringType)},
		{name: "unknown map", value: types.MapUnknown(types.StringType)},
		{name: "empty map", value: headers()},
		{name: "allows Accept", value: headers("Accept")},
		{name: "allows x-ms-version", value: headers("x-ms-version")},
		{name: "allows DataServiceVersion", value: headers("DataServiceVersion")},
		{name: "allows MaxDataServiceVersion", value: headers("MaxDataServiceVersion")},
		{name: "allows x-ms-client-request-id", value: headers("x-ms-client-request-id")},
		{name: "allows different case", value: headers("X-MS-VERSION", "accept")},
		{
			name:           "rejects Authorization",
			value:          headers("Authorization"),
			expectedErrors: []string{`header "Authorization" is not allowed; allowed headers: Accept, DataServiceVersion, MaxDataServiceVersion, x-ms-client-request-id, x-ms-version`},
		},
		{
			name:           "rejects Cookie",
			value:          headers("Cookie"),
			expectedErrors: []string{`header "Cookie" is not allowed; allowed headers: Accept, DataServiceVersion, MaxDataServiceVersion, x-ms-client-request-id, x-ms-version`},
		},
		{
			name:           "rejects Host",
			value:          headers("Host"),
			expectedErrors: []string{`header "Host" is not allowed; allowed headers: Accept, DataServiceVersion, MaxDataServiceVersion, x-ms-client-request-id, x-ms-version`},
		},
		{
			name:           "rejects x-ms-authorization-auxiliary",
			value:          headers("x-ms-authorization-auxiliary"),
			expectedErrors: []string{`header "x-ms-authorization-auxiliary" is not allowed; allowed headers: Accept, DataServiceVersion, MaxDataServiceVersion, x-ms-client-request-id, x-ms-version`},
		},
		{
			name:           "rejects arbitrary header",
			value:          headers("x-custom"),
			expectedErrors: []string{`header "x-custom" is not allowed; allowed headers: Accept, DataServiceVersion, MaxDataServiceVersion, x-ms-client-request-id, x-ms-version`},
		},
		{
			name:  "one sorted diagnostic per disallowed key",
			value: headers("x-custom", "x-ms-version", "Authorization", "Cookie"),
			expectedErrors: []string{
				`header "Authorization" is not allowed; allowed headers: Accept, DataServiceVersion, MaxDataServiceVersion, x-ms-client-request-id, x-ms-version`,
				`header "Cookie" is not allowed; allowed headers: Accept, DataServiceVersion, MaxDataServiceVersion, x-ms-client-request-id, x-ms-version`,
				`header "x-custom" is not allowed; allowed headers: Accept, DataServiceVersion, MaxDataServiceVersion, x-ms-client-request-id, x-ms-version`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := validator.MapRequest{
				ConfigValue: tc.value,
				Path:        path.Root("headers"),
			}
			resp := &validator.MapResponse{}

			v.ValidateMap(context.Background(), req, resp)

			if resp.Diagnostics.ErrorsCount() != len(tc.expectedErrors) {
				t.Fatalf("expected %d errors, got %d: %v", len(tc.expectedErrors), resp.Diagnostics.ErrorsCount(), resp.Diagnostics)
			}
			for i, diagnostic := range resp.Diagnostics.Errors() {
				if diagnostic.Detail() != tc.expectedErrors[i] {
					t.Fatalf("expected error %d to be %q, got %q", i, tc.expectedErrors[i], diagnostic.Detail())
				}
			}
		})
	}
}
