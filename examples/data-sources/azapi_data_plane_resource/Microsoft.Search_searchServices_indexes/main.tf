terraform {
  required_providers {
    azapi = {
      source = "Azure/azapi"
    }
  }
}

provider "azapi" {
}

resource "azapi_resource" "resourceGroup" {
  type     = "Microsoft.Resources/resourceGroups@2021-04-01"
  name     = "example-aisearch-dataplane"
  location = "westeurope"
}

resource "azapi_resource" "searchService" {
  type      = "Microsoft.Search/searchServices@2023-11-01"
  parent_id = azapi_resource.resourceGroup.id
  name      = "examplesearchdp"
  location  = azapi_resource.resourceGroup.location
  body = {
    properties = {
      replicaCount   = 1
      partitionCount = 1
      hostingMode    = "default"
      authOptions = {
        aadOrApiKey = {
          aadAuthFailureMode = "http401WithBearerChallenge"
        }
      }
    }
    sku = {
      name = "basic"
    }
  }
}

data "azapi_client_config" "current" {}

data "azapi_resource_list" "roleDefinitions" {
  type      = "Microsoft.Authorization/roleDefinitions@2022-04-01"
  parent_id = "/subscriptions/${data.azapi_client_config.current.subscription_id}"
  response_export_values = {
    searchServiceContributorRoleId = "value[?properties.roleName == 'Search Service Contributor'].id | [0]"
  }
}

resource "azapi_resource" "roleAssignment" {
  type      = "Microsoft.Authorization/roleAssignments@2022-04-01"
  parent_id = azapi_resource.searchService.id
  name      = uuid()
  body = {
    properties = {
      principalId      = data.azapi_client_config.current.object_id
      roleDefinitionId = data.azapi_resource_list.roleDefinitions.output.searchServiceContributorRoleId
    }
  }
  lifecycle {
    ignore_changes = [name]
  }
}

resource "azapi_data_plane_resource" "index" {
  type      = "Microsoft.Search/searchServices/indexes@2024-07-01"
  parent_id = "${azapi_resource.searchService.name}.search.windows.net"
  name      = "hotels-index"
  body = {
    fields = [
      {
        name       = "hotelId"
        type       = "Edm.String"
        key        = true
        searchable = false
      },
      {
        name       = "hotelName"
        type       = "Edm.String"
        searchable = true
      },
      {
        name       = "category"
        type       = "Edm.String"
        searchable = true
        filterable = true
      }
    ]
  }

  depends_on = [
    azapi_resource.roleAssignment,
  ]
}

data "azapi_data_plane_resource" "index" {
  type      = "Microsoft.Search/searchServices/indexes@2024-07-01"
  parent_id = "${azapi_resource.searchService.name}.search.windows.net"
  name      = azapi_data_plane_resource.index.name

  response_export_values = {
    field_names = "fields[].name"
    key_field   = "fields[?key].name | [0]"
  }
}

output "index_body" {
  value = data.azapi_data_plane_resource.index.body
}

output "index_field_names" {
  value = data.azapi_data_plane_resource.index.output.field_names
}

output "index_key_field" {
  value = data.azapi_data_plane_resource.index.output.key_field
}
