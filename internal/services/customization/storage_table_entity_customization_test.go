package customization

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
	"github.com/Azure/terraform-provider-azapi/internal/services/parse"
)

func TestBuildStorageTableEntityBodyAddsCompositeKeys(t *testing.T) {
	id := parse.DataPlaneResourceId{
		AzureResourceType: "Microsoft.Storage/storageAccounts/tableServices/tables/entities",
		AzureResourceId:   "mystorage.table.core.windows.net/mytable(PartitionKey='pk',RowKey='rk')",
	}

	body, err := buildStorageTableEntityBody(id, map[string]interface{}{
		"outputs": "value",
	})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if got := body["PartitionKey"]; got != "pk" {
		t.Fatalf("expected PartitionKey to be injected, got %#v", got)
	}
	if got := body["RowKey"]; got != "rk" {
		t.Fatalf("expected RowKey to be injected, got %#v", got)
	}
}

func TestBuildStorageTableEntityBodyRejectsMismatchedKeys(t *testing.T) {
	id := parse.DataPlaneResourceId{
		AzureResourceType: "Microsoft.Storage/storageAccounts/tableServices/tables/entities",
		AzureResourceId:   "mystorage.table.core.windows.net/mytable(PartitionKey='pk',RowKey='rk')",
	}

	_, err := buildStorageTableEntityBody(id, map[string]interface{}{
		"PartitionKey": "wrong",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), `PartitionKey "pk" in parent_id`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildStorageTableEntityBodyRejectsMissingKeys(t *testing.T) {
	id := parse.DataPlaneResourceId{
		AzureResourceType: "Microsoft.Storage/storageAccounts/tableServices/tables/entities",
		AzureResourceId:   "mystorage.table.core.windows.net/mytable",
		ParentId:          "mystorage.table.core.windows.net/mytable",
	}

	_, err := buildStorageTableEntityBody(id, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "must end with (PartitionKey=") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStorageTableEntityReadPreservesFullResponse(t *testing.T) {
	expected := map[string]interface{}{
		"PartitionKey": "pk",
		"RowKey":       "rk",
		"Timestamp":    "2026-09-13T00:00:00Z",
		"odata.etag":   `W/"datetime'2026-09-13T00%3A00%3A00.0000000Z'"`,
		"outputs":      "value",
	}
	dataPlaneClient, err := clients.NewDataPlaneClient(storageTableStaticTokenCredential{}, &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Cloud: cloud.AzurePublic,
			Transport: storageTableResponseTransport{
				body: `{
					"PartitionKey": "pk",
					"RowKey": "rk",
					"Timestamp": "2026-09-13T00:00:00Z",
					"odata.etag": "W/\"datetime'2026-09-13T00%3A00%3A00.0000000Z'\"",
					"outputs": "value"
				}`,
			},
		},
	})
	if err != nil {
		t.Fatalf("building data plane client: %v", err)
	}

	customization := StorageTableEntityCustomization{}
	result, err := customization.ReadFunc()(context.Background(), clients.Client{
		DataPlaneClient: dataPlaneClient,
	}, parse.DataPlaneResourceId{
		AzureResourceId: "management.azure.com/mytable(PartitionKey='pk',RowKey='rk')",
		ApiVersion:      "2026-04-06",
	}, clients.RequestOptions{})
	if err != nil {
		t.Fatalf("reading entity: %v", err)
	}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected full response %#v, got %#v", expected, result)
	}
}

type storageTableStaticTokenCredential struct{}

func (storageTableStaticTokenCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token:     "test-token",
		ExpiresOn: time.Now().Add(time.Hour),
	}, nil
}

type storageTableResponseTransport struct {
	body string
}

func (t storageTableResponseTransport) Do(request *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    io.NopCloser(strings.NewReader(t.body)),
		Request: request,
	}, nil
}
