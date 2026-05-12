package internal

import (
	"strings"
	"testing"

	pb "github.com/GoCodeAlone/workflow/plugin/external/proto"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// TestContractProviderInterface verifies the plugin implements
// sdk.ContractProvider so the engine routes configs through STRICT_PROTO
// (anypb.Any-wrapped typed messages) instead of legacy *structpb.Struct.
// This is the runtime check behind the Bug 3 fix: without this interface,
// CreateTypedModule receives an empty ProviderConfig and rejects the module
// with "config.provider is required".
func TestContractProviderInterface(t *testing.T) {
	var p sdk.PluginProvider = &paymentsPlugin{}
	if _, ok := p.(sdk.ContractProvider); !ok {
		t.Fatal("paymentsPlugin does not implement sdk.ContractProvider — Bug 3 will reappear")
	}
}

// TestContractRegistryCoversAllTypedModules ensures every module type advertised
// via TypedModuleTypes() has a matching STRICT_PROTO contract entry.  If a new
// module type is added but its contract descriptor is missed, the engine will
// silently fall back to legacy struct dispatch and the typed config path will
// be skipped.
func TestContractRegistryCoversAllTypedModules(t *testing.T) {
	p := &paymentsPlugin{}
	registry := p.ContractRegistry()
	if registry == nil {
		t.Fatal("ContractRegistry() returned nil")
	}

	moduleContracts := map[string]*pb.ContractDescriptor{}
	for _, c := range registry.Contracts {
		if c.Kind == pb.ContractKind_CONTRACT_KIND_MODULE {
			moduleContracts[c.ModuleType] = c
		}
	}

	for _, typeName := range p.TypedModuleTypes() {
		c, ok := moduleContracts[typeName]
		if !ok {
			t.Errorf("module type %q has no contract descriptor in ContractRegistry", typeName)
			continue
		}
		if c.Mode != pb.ContractMode_CONTRACT_MODE_STRICT_PROTO {
			t.Errorf("module %q: want CONTRACT_MODE_STRICT_PROTO, got %v", typeName, c.Mode)
		}
		if c.ConfigMessage == "" {
			t.Errorf("module %q: config_message is empty", typeName)
		}
		if !strings.HasPrefix(c.ConfigMessage, paymentsProtoPackage) {
			t.Errorf("module %q: config_message %q must be fully qualified under %q", typeName, c.ConfigMessage, paymentsProtoPackage)
		}
	}
}

// TestContractRegistryCoversAllTypedSteps ensures every step type advertised
// via TypedStepTypes() has a matching STRICT_PROTO contract entry with all
// three of config/input/output messages populated.
func TestContractRegistryCoversAllTypedSteps(t *testing.T) {
	p := &paymentsPlugin{}
	registry := p.ContractRegistry()
	if registry == nil {
		t.Fatal("ContractRegistry() returned nil")
	}

	stepContracts := map[string]*pb.ContractDescriptor{}
	for _, c := range registry.Contracts {
		if c.Kind == pb.ContractKind_CONTRACT_KIND_STEP {
			stepContracts[c.StepType] = c
		}
	}

	for _, typeName := range p.TypedStepTypes() {
		c, ok := stepContracts[typeName]
		if !ok {
			t.Errorf("step type %q has no contract descriptor in ContractRegistry", typeName)
			continue
		}
		if c.Mode != pb.ContractMode_CONTRACT_MODE_STRICT_PROTO {
			t.Errorf("step %q: want CONTRACT_MODE_STRICT_PROTO, got %v", typeName, c.Mode)
		}
		if c.ConfigMessage == "" {
			t.Errorf("step %q: config_message is empty", typeName)
		}
		if c.InputMessage == "" {
			t.Errorf("step %q: input_message is empty", typeName)
		}
		if c.OutputMessage == "" {
			t.Errorf("step %q: output_message is empty", typeName)
		}
		for _, m := range []string{c.ConfigMessage, c.InputMessage, c.OutputMessage} {
			if m != "" && !strings.HasPrefix(m, paymentsProtoPackage) {
				t.Errorf("step %q: message %q must be fully qualified under %q", typeName, m, paymentsProtoPackage)
			}
		}
	}
}

// TestContractRegistryFileDescriptorSet verifies the embedded
// FileDescriptorSet includes the payments.v1 proto file so the engine can
// resolve message names without a separate registry lookup.
func TestContractRegistryFileDescriptorSet(t *testing.T) {
	p := &paymentsPlugin{}
	registry := p.ContractRegistry()
	if registry == nil || registry.FileDescriptorSet == nil {
		t.Fatal("ContractRegistry.FileDescriptorSet is nil")
	}

	const wantFile = "proto/payments/v1/payments.proto"
	var found bool
	for _, f := range registry.FileDescriptorSet.File {
		if f.GetName() == wantFile {
			found = true
			if got := f.GetPackage(); got != "workflow.plugins.payments.v1" {
				t.Errorf("payments proto package mismatch: want workflow.plugins.payments.v1, got %s", got)
			}
			break
		}
	}
	if !found {
		t.Errorf("FileDescriptorSet does not contain %q", wantFile)
	}
}
