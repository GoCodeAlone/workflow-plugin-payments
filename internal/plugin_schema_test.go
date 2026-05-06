package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

func TestModuleSchemas_ReturnsOneSchema(t *testing.T) {
	moduleSchema(t)
}

func TestModuleSchemas_ProviderType(t *testing.T) {
	schema := moduleSchema(t)
	if schema.Type != "payments.provider" {
		t.Errorf("expected type %q, got %q", "payments.provider", schema.Type)
	}
}

func TestModuleSchemas_RequiredFields(t *testing.T) {
	schema := moduleSchema(t)

	fieldMap := make(map[string]bool)
	for _, f := range schema.ConfigFields {
		fieldMap[f.Name] = true
	}

	// provider field must be present and required
	found := false
	for _, f := range schema.ConfigFields {
		if f.Name == "provider" {
			found = true
			if !f.Required {
				t.Error("provider field must be required")
			}
			break
		}
	}
	if !found {
		t.Error("provider field missing from module schema")
	}

	// All expected fields must be present
	expectedFields := []string{
		"provider",
		"secretKey", "webhookSecret", "defaultCurrency",
		"clientId", "clientSecret", "environment", "webhookId",
	}
	for _, name := range expectedFields {
		if !fieldMap[name] {
			t.Errorf("expected field %q missing from module schema", name)
		}
	}
}

func TestModuleSchemas_ProviderOptions(t *testing.T) {
	schema := moduleSchema(t)

	for _, f := range schema.ConfigFields {
		if f.Name == "provider" {
			optSet := make(map[string]bool)
			for _, o := range f.Options {
				optSet[o] = true
			}
			if !optSet["stripe"] {
				t.Error("provider field must include 'stripe' option")
			}
			if !optSet["paypal"] {
				t.Error("provider field must include 'paypal' option")
			}
			return
		}
	}
	t.Error("provider field not found")
}

func TestModuleSchemas_EnvironmentOptions(t *testing.T) {
	schema := moduleSchema(t)

	for _, f := range schema.ConfigFields {
		if f.Name == "environment" {
			optSet := make(map[string]bool)
			for _, o := range f.Options {
				optSet[o] = true
			}
			if !optSet["sandbox"] {
				t.Error("environment field must include 'sandbox' option")
			}
			if !optSet["production"] {
				t.Error("environment field must include 'production' option")
			}
			if !optSet["live"] {
				t.Error("environment field must include 'live' option (alias for production)")
			}
			if f.DefaultValue != "sandbox" {
				t.Errorf("environment default must be 'sandbox', got %q", f.DefaultValue)
			}
			return
		}
	}
	t.Error("environment field not found")
}

func TestModuleSchemas_DefaultCurrency(t *testing.T) {
	schema := moduleSchema(t)

	for _, f := range schema.ConfigFields {
		if f.Name == "defaultCurrency" {
			if f.DefaultValue != "usd" {
				t.Errorf("defaultCurrency default must be 'usd', got %q", f.DefaultValue)
			}
			return
		}
	}
	t.Error("defaultCurrency field not found")
}

func TestModuleSchemas_DocumentsProviderRequirementsAndAliases(t *testing.T) {
	schema := moduleSchema(t)

	fields := make(map[string]sdk.ConfigField)
	for _, f := range schema.ConfigFields {
		fields[f.Name] = f
	}

	cases := []struct {
		field string
		want  []string
	}{
		{field: "secretKey", want: []string{"secret_key"}},
		{field: "webhookSecret", want: []string{"webhook_secret"}},
		{field: "defaultCurrency", want: []string{"default_currency"}},
		{field: "clientId", want: []string{"Required when provider=paypal", "client_id"}},
		{field: "clientSecret", want: []string{"Required when provider=paypal", "client_secret"}},
		{field: "webhookId", want: []string{"webhook_id"}},
	}

	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			field, ok := fields[tc.field]
			if !ok {
				t.Fatalf("field %q missing from module schema", tc.field)
			}
			for _, want := range tc.want {
				if !strings.Contains(field.Description, want) {
					t.Errorf("field %q description %q does not mention %q", tc.field, field.Description, want)
				}
			}
		})
	}
}

func TestPluginManifest_ErrorOutputsDocumentOmittedOnSuccess(t *testing.T) {
	var manifest struct {
		StepSchemas []struct {
			Type    string `json:"type"`
			Outputs []struct {
				Key         string `json:"key"`
				Description string `json:"description"`
			} `json:"outputs"`
		} `json:"stepSchemas"`
	}
	readJSONFile(t, filepath.Join("..", "plugin.json"), &manifest)

	for _, step := range manifest.StepSchemas {
		found := false
		for _, output := range step.Outputs {
			if output.Key != "error" {
				continue
			}
			found = true
			if !strings.Contains(output.Description, "omitted on success") {
				t.Errorf("%s error output description %q does not document omission on success", step.Type, output.Description)
			}
			if strings.Contains(output.Description, "empty on success") {
				t.Errorf("%s error output description %q still says empty on success", step.Type, output.Description)
			}
		}
		if !found {
			t.Errorf("%s does not document an error output", step.Type)
		}
	}
}

func TestCIWorkflow_ValidatesContractsFileWithStrictWfctl(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "ci.yml")
	b, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read CI workflow: %v", err)
	}
	workflow := string(b)

	// Check high-level properties without pinning exact command strings or versions
	// so minor refactors (job rename, wfctl version bump, formatting) don't break this test.
	required := []string{
		"plugin-contracts",      // dedicated job must exist
		"--strict-contracts",    // strict validation flag must be present
		"plugin.contracts.json", // contracts file must be checked
	}
	for _, want := range required {
		if !strings.Contains(workflow, want) {
			t.Errorf("CI workflow does not contain %q", want)
		}
	}
}

func moduleSchema(t *testing.T) sdk.ModuleSchemaData {
	t.Helper()
	p := &paymentsPlugin{}
	schemas := p.ModuleSchemas()
	if len(schemas) != 1 {
		t.Fatalf("expected 1 module schema, got %d", len(schemas))
	}
	return schemas[0]
}

func readJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}
