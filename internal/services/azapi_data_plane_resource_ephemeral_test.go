package services_test

import (
	"fmt"
	"testing"

	"github.com/Azure/terraform-provider-azapi/internal/acceptance"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

type DataPlaneEphemeral struct{}

func TestAccEphemeralDataPlaneResource_appConfigKeyValues(t *testing.T) {
	data := acceptance.BuildTestData(t, "ephemeral.azapi_data_plane_resource", "test")
	r := DataPlaneEphemeral{}

	data.DataSourceTest(t, []resource.TestStep{
		{
			Config:            r.appConfigKeyValues(data),
			ExternalProviders: externalProvidersAzurerm(),
			Check:             resource.ComposeTestCheckFunc(),
		},
	})
}

func (r DataPlaneEphemeral) appConfigKeyValues(data acceptance.TestData) string {
	return fmt.Sprintf(`
provider "azurerm" {
  features {}
}

resource "azurerm_resource_group" "example" {
  name     = "acctest%[2]s"
  location = "%[1]s"
}

resource "azurerm_app_configuration" "appconf" {
  name                = "acctest%[2]s"
  resource_group_name = azurerm_resource_group.example.name
  location            = azurerm_resource_group.example.location
  sku                 = "standard"
}

data "azurerm_client_config" "current" {}

resource "azurerm_role_assignment" "appconf" {
  scope                = azurerm_app_configuration.appconf.id
  role_definition_name = "App Configuration Data Owner"
  principal_id         = data.azurerm_client_config.current.object_id
}

resource "azapi_data_plane_resource" "setup" {
  type      = "Microsoft.AppConfiguration/configurationStores/keyValues@1.0"
  parent_id = replace(azurerm_app_configuration.appconf.endpoint, "https://", "")
  name      = "mykey"
  body = {
    content_type = ""
    value        = "myvalue"
  }

  depends_on = [
    azurerm_role_assignment.appconf,
  ]
}

ephemeral "azapi_data_plane_resource" "test" {
  type      = "Microsoft.AppConfiguration/configurationStores/keyValues@1.0"
  parent_id = replace(azurerm_app_configuration.appconf.endpoint, "https://", "")
  name      = "mykey"

  response_export_values = ["*"]

  depends_on = [
    azapi_data_plane_resource.setup,
  ]
}
`, data.LocationPrimary, data.RandomString)
}
