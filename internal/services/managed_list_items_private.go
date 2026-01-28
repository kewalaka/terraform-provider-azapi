package services

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

const (
	pkManagedListItems = "managed_list_items"
)

// ManagedListItemsPrivateMgr tracks the identifiers of list items that were in the
// Terraform configuration during the last apply. This enables distinguishing between:
// - Items added server-side (not previously managed, should be preserved)
// - Items removed from config (previously managed, should be removed)
//
// This is used in conjunction with ignore_other_items_in_list to ensure that
// removing an item from config actually removes it from Azure, rather than
// treating it as a server-side item to preserve.
type ManagedListItemsPrivateMgr struct{}

var managedListItemsPrivateMgr = ManagedListItemsPrivateMgr{}

// Get retrieves the previously managed list items from private state.
// Returns nil map if no items were previously stored (e.g., first apply or feature just enabled).
func (m ManagedListItemsPrivateMgr) Get(ctx context.Context, d PrivateData) (map[string][]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	b, diags := d.GetKey(ctx, pkManagedListItems)
	if diags.HasError() {
		return nil, diags
	}
	if b == nil {
		return nil, diags
	}

	var items map[string][]string
	if err := json.Unmarshal(b, &items); err != nil {
		diags.AddError(
			`Error unmarshalling "managed_list_items" private data`,
			err.Error(),
		)
		return nil, diags
	}

	return items, diags
}

// Set stores the managed list item identifiers in private state.
// The items map is keyed by list path (e.g., "properties.logs") with values
// being the list of item identifiers at that path.
// Pass nil to clear the stored items.
func (m ManagedListItemsPrivateMgr) Set(ctx context.Context, d PrivateData, items map[string][]string) diag.Diagnostics {
	if len(items) == 0 {
		return d.SetKey(ctx, pkManagedListItems, nil)
	}

	b, err := json.Marshal(items)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError(
			`Error marshalling "managed_list_items" private data`,
			err.Error(),
		)
		return diags
	}

	return d.SetKey(ctx, pkManagedListItems, b)
}
