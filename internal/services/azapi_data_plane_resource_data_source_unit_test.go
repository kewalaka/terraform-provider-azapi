package services

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/terraform-provider-azapi/internal/clients"
	"github.com/Azure/terraform-provider-azapi/internal/retry"
	"github.com/Azure/terraform-provider-azapi/internal/services/parse"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDataPlaneResourceDataSourceSchemaHeaders(t *testing.T) {
	var response datasource.SchemaResponse
	dataSource := DataPlaneResourceDataSource{}
	dataSource.Schema(context.Background(), datasource.SchemaRequest{}, &response)

	headers, ok := response.Schema.Attributes["headers"].(schema.MapAttribute)
	if !ok || !headers.IsOptional() || headers.IsRequired() || headers.IsComputed() || headers.ElementType != types.StringType {
		t.Fatalf("expected optional map(string) headers attribute, got %#v", response.Schema.Attributes["headers"])
	}
	if _, ok := response.Schema.Attributes["query_parameters"]; ok {
		t.Fatal("expected data source to not expose query_parameters")
	}

	validate := func(keys ...string) validator.MapResponse {
		elements := make(map[string]attr.Value, len(keys))
		for _, key := range keys {
			elements[key] = types.StringValue("value")
		}
		var resp validator.MapResponse
		for _, v := range headers.MapValidators() {
			v.ValidateMap(context.Background(), validator.MapRequest{
				Path:        path.Root("headers"),
				ConfigValue: types.MapValueMust(types.StringType, elements),
			}, &resp)
		}
		return resp
	}
	if resp := validate(DataPlaneReadHeaderAllowList...); resp.Diagnostics.HasError() {
		t.Fatalf("expected allow-listed headers to be accepted, got %v", resp.Diagnostics)
	}
	if resp := validate("Authorization"); resp.Diagnostics.ErrorsCount() != 1 {
		t.Fatalf("expected Authorization header to be rejected, got %v", resp.Diagnostics)
	}
}

func TestDataPlaneResourceDataSourceRequestOptions(t *testing.T) {
	model := &DataPlaneResourceDataSourceModel{
		Headers: types.MapValueMust(types.StringType, map[string]attr.Value{
			"x-ms-version":           types.StringValue("2026-04-06"),
			"x-ms-client-request-id": types.StringValue("request-id"),
		}),
		Retry: retry.NewRetryValueNull(),
	}

	options := dataPlaneResourceDataSourceRequestOptions(model)

	expectedHeaders := map[string]string{
		"x-ms-version":           "2026-04-06",
		"x-ms-client-request-id": "request-id",
	}
	if !reflect.DeepEqual(options.Headers, expectedHeaders) {
		t.Fatalf("expected headers %#v, got %#v", expectedHeaders, options.Headers)
	}
	if len(options.QueryParameters) != 0 {
		t.Fatalf("expected no query parameters, got %#v", options.QueryParameters)
	}
	if options.RetryOptions != nil || options.LastRetryError != nil {
		t.Fatal("expected null retry configuration to remain disabled")
	}

	transport := &dataSourceHeadersCaptureTransport{}
	dataPlaneClient, err := clients.NewDataPlaneClient(dataSourceHeadersStaticTokenCredential{}, &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Cloud:     cloud.AzurePublic,
			Transport: transport,
		},
	})
	if err != nil {
		t.Fatalf("building data plane client: %v", err)
	}
	if _, err := dataPlaneClient.Get(context.Background(), parse.DataPlaneResourceId{
		AzureResourceId: "management.azure.com/mytable(PartitionKey='pk',RowKey='rk')",
		ApiVersion:      "2026-04-06",
	}, options); err != nil {
		t.Fatalf("sending request: %v", err)
	}
	if transport.request == nil {
		t.Fatal("expected a request to be sent")
	}
	if got := transport.request.Header.Get("x-ms-version"); got != "2026-04-06" {
		t.Fatalf("expected x-ms-version header, got %q", got)
	}
	if got := transport.request.Header.Get("x-ms-client-request-id"); got != "request-id" {
		t.Fatalf("expected x-ms-client-request-id header, got %q", got)
	}
}

func TestDataPlaneResourceDataSourceCannotOverrideAuthorization(t *testing.T) {
	transport := &dataSourceHeadersCaptureTransport{}
	dataPlaneClient, err := clients.NewDataPlaneClient(dataSourceHeadersStaticTokenCredential{}, &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Cloud:     cloud.AzurePublic,
			Transport: transport,
		},
	})
	if err != nil {
		t.Fatalf("building data plane client: %v", err)
	}
	if _, err := dataPlaneClient.Get(context.Background(), parse.DataPlaneResourceId{
		AzureResourceId: "management.azure.com/mytable(PartitionKey='pk',RowKey='rk')",
		ApiVersion:      "2026-04-06",
	}, clients.RequestOptions{
		Headers: map[string]string{"Authorization": "Bearer attacker"},
	}); err != nil {
		t.Fatalf("sending request: %v", err)
	}
	if transport.request == nil {
		t.Fatal("expected a request to be sent")
	}
	if got := transport.request.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Fatalf("expected bearer token policy to overwrite Authorization, got %q", got)
	}
}

func TestDataPlaneResourceDataSourceRequestOptionsNullHeaders(t *testing.T) {
	options := dataPlaneResourceDataSourceRequestOptions(&DataPlaneResourceDataSourceModel{
		Headers: types.MapNull(types.StringType),
		Retry:   retry.NewRetryValueNull(),
	})
	if len(options.Headers) != 0 {
		t.Fatalf("expected no headers for null configuration, got %#v", options.Headers)
	}
}

type dataSourceHeadersStaticTokenCredential struct{}

func (dataSourceHeadersStaticTokenCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token:     "test-token",
		ExpiresOn: time.Now().Add(time.Hour),
	}, nil
}

type dataSourceHeadersCaptureTransport struct {
	request *http.Request
}

func (t *dataSourceHeadersCaptureTransport) Do(request *http.Request) (*http.Response, error) {
	t.request = request
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    io.NopCloser(strings.NewReader(`{}`)),
		Request: request,
	}, nil
}
