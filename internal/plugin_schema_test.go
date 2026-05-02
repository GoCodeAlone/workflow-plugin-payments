package internal

import (
	"testing"
)

func TestModuleSchemas_ReturnsOneSchema(t *testing.T) {
	p := &paymentsPlugin{}
	schemas := p.ModuleSchemas()
	if len(schemas) != 1 {
		t.Fatalf("expected 1 module schema, got %d", len(schemas))
	}
}

func TestModuleSchemas_ProviderType(t *testing.T) {
	p := &paymentsPlugin{}
	schema := p.ModuleSchemas()[0]
	if schema.Type != "payments.provider" {
		t.Errorf("expected type %q, got %q", "payments.provider", schema.Type)
	}
}

func TestModuleSchemas_RequiredFields(t *testing.T) {
	p := &paymentsPlugin{}
	schema := p.ModuleSchemas()[0]

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
	p := &paymentsPlugin{}
	schema := p.ModuleSchemas()[0]

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
	p := &paymentsPlugin{}
	schema := p.ModuleSchemas()[0]

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
	p := &paymentsPlugin{}
	schema := p.ModuleSchemas()[0]

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
