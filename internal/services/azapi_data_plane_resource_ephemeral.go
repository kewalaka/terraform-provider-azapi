package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/terraform-provider-azapi/internal/clients"
	"github.com/Azure/terraform-provider-azapi/internal/docstrings"
	"github.com/Azure/terraform-provider-azapi/internal/retry"
	"github.com/Azure/terraform-provider-azapi/internal/services/common"
	"github.com/Azure/terraform-provider-azapi/internal/services/customization"
	"github.com/Azure/terraform-provider-azapi/internal/services/dynamic"
	"github.com/Azure/terraform-provider-azapi/internal/services/myvalidator"
	"github.com/Azure/terraform-provider-azapi/internal/services/parse"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/ephemeral/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	tffwdocs "github.com/magodo/terraform-plugin-framework-docs"
)

type DataPlaneResourceEphemeralModel struct {
	ID                   types.String     `tfsdk:"id"`
	Name                 types.String     `tfsdk:"name"`
	ParentID             types.String     `tfsdk:"parent_id"`
	Type                 types.String     `tfsdk:"type"`
	Identifiers          types.Map        `tfsdk:"identifiers"`
	Body                 types.Dynamic    `tfsdk:"body"`
	ResponseExportValues types.Dynamic    `tfsdk:"response_export_values"`
	Output               types.Dynamic    `tfsdk:"output"`
	Timeouts             timeouts.Value   `tfsdk:"timeouts"`
	Retry                retry.RetryValue `tfsdk:"retry"`
	Headers              types.Map        `tfsdk:"headers"`
	QueryParameters      types.Map        `tfsdk:"query_parameters"`
}

type DataPlaneResourceEphemeral struct {
	ProviderData *clients.Client
}

var _ ephemeral.EphemeralResource = &DataPlaneResourceEphemeral{}
var _ ephemeral.EphemeralResourceWithConfigure = &DataPlaneResourceEphemeral{}
var _ ephemeral.EphemeralResourceWithValidateConfig = &DataPlaneResourceEphemeral{}
var _ tffwdocs.EphemeralResourceWithRenderOption = &DataPlaneResourceEphemeral{}

func (r *DataPlaneResourceEphemeral) Metadata(ctx context.Context, request ephemeral.MetadataRequest, response *ephemeral.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_data_plane_resource"
}

func (r *DataPlaneResourceEphemeral) Configure(ctx context.Context, request ephemeral.ConfigureRequest, response *ephemeral.ConfigureResponse) {
	if v, ok := request.ProviderData.(*clients.Client); ok {
		r.ProviderData = v
	}
}

func (r *DataPlaneResourceEphemeral) ValidateConfig(ctx context.Context, request ephemeral.ValidateConfigRequest, response *ephemeral.ValidateConfigResponse) {
	var config *DataPlaneResourceEphemeralModel
	if response.Diagnostics.Append(request.Config.Get(ctx, &config)...); response.Diagnostics.HasError() {
		return
	}
	if config == nil {
		return
	}

	resourceConfig := &DataPlaneResourceModel{
		Name:        config.Name,
		ParentID:    config.ParentID,
		Type:        config.Type,
		Identifiers: config.Identifiers,
	}
	if err := validateDataPlaneResourceIdentifier(resourceConfig); err != nil {
		response.Diagnostics.AddError("Invalid configuration", err.Error())
	}
}

func (r *DataPlaneResourceEphemeral) Schema(ctx context.Context, request ephemeral.SchemaRequest, response *ephemeral.SchemaResponse) {
	response.Schema = schema.Schema{
		MarkdownDescription: "This ephemeral resource can read Azure data plane resources without persisting state. It is useful for retrieving sensitive data plane resource content, such as secrets or tokens, that should not be stored in the Terraform state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: docstrings.ID(),
			},
			"name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Specifies the name (identifier segment) of the data plane resource when the selected resource type uses a single `name` path segment.",
			},
			"parent_id": schema.StringAttribute{
				Required:            true,
				Validators:          []validator.String{myvalidator.StringIsNotEmpty()},
				MarkdownDescription: "The parent ID or endpoint prefix for the data plane resource being read.",
			},
			"type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					myvalidator.StringIsResourceType(),
				},
				MarkdownDescription: docstrings.Type(),
			},
			"identifiers": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "A mapping of identifier placeholder values for data plane resource types that require multiple path identifiers, for example composite keys.",
			},
			"body": schema.DynamicAttribute{
				Computed:            true,
				MarkdownDescription: docstrings.Body(),
			},
			"response_export_values": schema.DynamicAttribute{
				Optional:            true,
				MarkdownDescription: docstrings.ResponseExportValues(),
			},
			"output": schema.DynamicAttribute{
				Computed:            true,
				MarkdownDescription: docstrings.Output("ephemeral.azapi_data_plane_resource"),
			},
			"retry": retry.RetryEphemeralSchema(ctx),
			"headers": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				MarkdownDescription: "A map of headers to include in the request.",
			},
			"query_parameters": schema.MapAttribute{
				ElementType: types.ListType{
					ElemType: types.StringType,
				},
				Optional:            true,
				MarkdownDescription: "A map of query parameters to include in the request.",
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx),
		},
	}
}

func (r *DataPlaneResourceEphemeral) Open(ctx context.Context, request ephemeral.OpenRequest, response *ephemeral.OpenResponse) {
	var model DataPlaneResourceEphemeralModel
	if response.Diagnostics.Append(request.Config.Get(ctx, &model)...); response.Diagnostics.HasError() {
		return
	}

	readTimeout, diags := model.Timeouts.Open(ctx, 5*time.Minute)
	if response.Diagnostics.Append(diags...); response.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()

	id, err := parse.NewDataPlaneResourceIdWithIdentifiers(model.Name.ValueString(), model.ParentID.ValueString(), model.Type.ValueString(), common.AsMapOfString(model.Identifiers))
	if err != nil {
		response.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}
	ctx = tflog.SetField(ctx, "resource_id", id.ID())

	requestOptions := clients.RequestOptions{
		Headers:         common.AsMapOfString(model.Headers),
		QueryParameters: clients.NewQueryParameters(common.AsMapOfLists(model.QueryParameters)),
	}
	requestOptions.RetryOptions, requestOptions.LastRetryError = clients.NewRetryOptions(model.Retry)

	var responseBody interface{}
	if customizedResource := customization.GetCustomization(model.Type.ValueString()); customizedResource != nil && (*customizedResource).ReadFunc() != nil {
		responseBody, err = (*customizedResource).ReadFunc()(ctx, *r.ProviderData, id, requestOptions)
	} else {
		responseBody, err = r.ProviderData.DataPlaneClient.Get(ctx, id, requestOptions)
	}
	if err != nil {
		response.Diagnostics.AddError("Failed to retrieve resource", fmt.Errorf("reading %s: %+v", id, err).Error())
		return
	}

	bodyData, err := json.Marshal(responseBody)
	if err != nil {
		response.Diagnostics.AddError("Invalid body", err.Error())
		return
	}
	body, err := dynamic.FromJSONImplied(bodyData)
	if err != nil {
		response.Diagnostics.AddError("Invalid body", err.Error())
		return
	}

	output, err := buildOutputFromBody(responseBody, model.ResponseExportValues, responseBody)
	if err != nil {
		response.Diagnostics.AddError("Failed to build output", err.Error())
		return
	}

	model.ID = basetypes.NewStringValue(id.ID())
	model.Name = basetypes.NewStringValue(id.Name)
	model.ParentID = basetypes.NewStringValue(id.ParentId)
	model.Type = basetypes.NewStringValue(fmt.Sprintf("%s@%s", id.AzureResourceType, id.ApiVersion))
	model.Identifiers = stringMapToTypesMap(id.Identifiers)
	model.Body = body
	model.Output = output

	response.Diagnostics.Append(response.Result.Set(ctx, model)...)
}

func (r *DataPlaneResourceEphemeral) RenderOption() tffwdocs.EphemeralResourceRenderOption {
	return tffwdocs.EphemeralResourceRenderOption{
		Examples: []tffwdocs.Example{
			{
				HCL: `
terraform {
  required_providers {
    azapi = {
      source = "Azure/azapi"
    }
  }
}

provider "azapi" {
}

ephemeral "azapi_data_plane_resource" "example" {
  type      = "Microsoft.AppConfiguration/configurationStores/keyValues@1.0"
  name      = "mykey"
  parent_id = "https://mystore.azconfig.io"
  response_export_values = {
    all = "@"
  }
}
`,
			},
		},
	}
}
