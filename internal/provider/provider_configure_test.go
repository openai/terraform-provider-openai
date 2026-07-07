package provider

import (
	"context"
	"testing"

	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	openaiapi "github.com/openai/terraform-provider-openai/internal/provider/openaiapi"
)

func TestOpenAIProviderConfigureBaseURL(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		baseURL  any
		expected string
	}{
		"default": {
			baseURL:  nil,
			expected: defaultBaseURL,
		},
		"configured": {
			baseURL:  "https://platform.api.openai.org/v1",
			expected: "https://platform.api.openai.org/v1",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			providerUnderTest := &OpenAIProvider{version: "test"}
			ctx := context.Background()
			var schemaResponse frameworkprovider.SchemaResponse
			providerUnderTest.Schema(ctx, frameworkprovider.SchemaRequest{}, &schemaResponse)

			attributeTypes := map[string]tftypes.Type{
				"admin_api_key": tftypes.String,
				"base_url":      tftypes.String,
				"organization":  tftypes.String,
				"project":       tftypes.String,
			}
			config := tfsdk.Config{
				Raw: tftypes.NewValue(
					tftypes.Object{AttributeTypes: attributeTypes},
					map[string]tftypes.Value{
						"admin_api_key": tftypes.NewValue(tftypes.String, "test-key"),
						"base_url":      tftypes.NewValue(tftypes.String, testCase.baseURL),
						"organization":  tftypes.NewValue(tftypes.String, nil),
						"project":       tftypes.NewValue(tftypes.String, nil),
					},
				),
				Schema: schemaResponse.Schema,
			}
			var configureResponse frameworkprovider.ConfigureResponse
			providerUnderTest.Configure(ctx, frameworkprovider.ConfigureRequest{Config: config}, &configureResponse)

			if configureResponse.Diagnostics.HasError() {
				t.Fatalf("unexpected configure diagnostics: %v", configureResponse.Diagnostics)
			}
			client, ok := configureResponse.ResourceData.(*openaiapi.APIClient)
			if !ok {
				t.Fatalf("expected *openaiapi.APIClient, got %T", configureResponse.ResourceData)
			}
			if client.BaseURL != testCase.expected {
				t.Fatalf("expected base URL %q, got %q", testCase.expected, client.BaseURL)
			}
		})
	}
}

func TestOpenAIProviderConfigureRejectsEmptyBaseURL(t *testing.T) {
	t.Parallel()

	providerUnderTest := &OpenAIProvider{version: "test"}
	ctx := context.Background()
	var schemaResponse frameworkprovider.SchemaResponse
	providerUnderTest.Schema(ctx, frameworkprovider.SchemaRequest{}, &schemaResponse)

	attributeTypes := map[string]tftypes.Type{
		"admin_api_key": tftypes.String,
		"base_url":      tftypes.String,
		"organization":  tftypes.String,
		"project":       tftypes.String,
	}
	config := tfsdk.Config{
		Raw: tftypes.NewValue(
			tftypes.Object{AttributeTypes: attributeTypes},
			map[string]tftypes.Value{
				"admin_api_key": tftypes.NewValue(tftypes.String, "test-key"),
				"base_url":      tftypes.NewValue(tftypes.String, "  "),
				"organization":  tftypes.NewValue(tftypes.String, nil),
				"project":       tftypes.NewValue(tftypes.String, nil),
			},
		),
		Schema: schemaResponse.Schema,
	}
	var configureResponse frameworkprovider.ConfigureResponse
	providerUnderTest.Configure(ctx, frameworkprovider.ConfigureRequest{Config: config}, &configureResponse)

	if !configureResponse.Diagnostics.HasError() {
		t.Fatal("expected empty base_url to produce an error")
	}
}
