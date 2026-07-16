package parse

import (
	"reflect"
	"strings"
	"testing"
)

func TestPlaceholdersForURLFormat(t *testing.T) {
	urlFormat := "{parentId}/resourceSetRuleConfigs/{name=defaultResourceSetRuleConfig}/{tokenId}"
	got := placeholdersForURLFormat(urlFormat)

	want := []dataPlanePlaceholder{
		{Key: "parentId"},
		{Key: "name", HasDefault: true, DefaultValue: "defaultResourceSetRuleConfig"},
		{Key: "tokenId"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected placeholders %#v, got %#v", want, got)
	}
}

func TestRenderDataPlaneURLFormat(t *testing.T) {
	t.Run("renders URL with required placeholders", func(t *testing.T) {
		url, err := renderDataPlaneURLFormat(
			"{parentId}/{tableName}(PartitionKey='{partitionKey}',RowKey='{rowKey}')",
			map[string]string{
				"parentId":     "acct.table.core.windows.net",
				"tableName":    "mytable",
				"partitionKey": "pk",
				"rowKey":       "rk",
			},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "acct.table.core.windows.net/mytable(PartitionKey='pk',RowKey='rk')"
		if url != want {
			t.Fatalf("expected %q, got %q", want, url)
		}
	})

	t.Run("uses default placeholder value", func(t *testing.T) {
		url, err := renderDataPlaneURLFormat(
			"{parentId}/resourceSetRuleConfigs/{name=defaultResourceSetRuleConfig}",
			map[string]string{"parentId": "mypurview.purview.azure.com"},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "mypurview.purview.azure.com/resourceSetRuleConfigs/defaultResourceSetRuleConfig"
		if url != want {
			t.Fatalf("expected %q, got %q", want, url)
		}
	})

	t.Run("returns error on missing required values", func(t *testing.T) {
		_, err := renderDataPlaneURLFormat("{parentId}/kv/{name}", map[string]string{"parentId": "example.azconfig.io"})
		if err == nil {
			t.Fatal("expected error for missing required placeholder")
		}
		if !strings.Contains(err.Error(), "name") {
			t.Fatalf("expected missing placeholder in error, got %v", err)
		}
	})
}

func TestParseDataPlaneURLFormat(t *testing.T) {
	values, err := parseDataPlaneURLFormat(
		"{parentId}/resourceSetRuleConfigs/{name=defaultResourceSetRuleConfig}",
		"mypurview.purview.azure.com/resourceSetRuleConfigs/defaultResourceSetRuleConfig",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if values["parentId"] != "mypurview.purview.azure.com" {
		t.Fatalf("unexpected parentId: %#v", values)
	}
	if values["name"] != "defaultResourceSetRuleConfig" {
		t.Fatalf("unexpected name: %#v", values)
	}
}

func TestDataPlaneResourcePlaceholderKeys(t *testing.T) {
	t.Run("returns non-default placeholders excluding parent and api version", func(t *testing.T) {
		keys, err := DataPlaneResourcePlaceholderKeys("Microsoft.Storage/storageAccounts/tableServices/tables/entities@2026-04-06")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"partitionKey", "rowKey"}
		if !reflect.DeepEqual(keys, want) {
			t.Fatalf("expected keys %#v, got %#v", want, keys)
		}
	})

	t.Run("returns empty keys for singleton with default name", func(t *testing.T) {
		keys, err := DataPlaneResourcePlaceholderKeys("Microsoft.Purview/accounts/Account/resourceSetRuleConfigs@2021-07-01")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(keys) != 0 {
			t.Fatalf("expected no required placeholders, got %#v", keys)
		}
	})
}
