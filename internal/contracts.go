package internal

import (
	paymentsv1 "github.com/GoCodeAlone/workflow-plugin-payments/proto/payments/v1"
	pb "github.com/GoCodeAlone/workflow/plugin/external/proto"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/structpb"
)

// Compile-time check that paymentsPlugin satisfies sdk.ContractProvider.
//
// Bug 3 fix: without ContractProvider, the engine cannot discover that
// payments.provider has a STRICT_PROTO config message, so it falls back to
// dispatching *structpb.Struct.  CreateTypedModule then unmarshals into a
// zero-value ProviderConfig and rejects the module with
// "config.provider is required".
var _ sdk.ContractProvider = (*paymentsPlugin)(nil)

// ContractRegistry returns the typed-contract descriptors exposed by this
// plugin.  The engine uses this to route module/step configs through the
// strict-proto path (anypb.Any wrapping the typed message) instead of the
// legacy struct path (*structpb.Struct).
func (p *paymentsPlugin) ContractRegistry() *pb.ContractRegistry {
	return paymentsContractRegistry
}

var paymentsContractRegistry = &pb.ContractRegistry{
	FileDescriptorSet: &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			protodesc.ToFileDescriptorProto(structpb.File_google_protobuf_struct_proto),
			protodesc.ToFileDescriptorProto(paymentsv1.File_proto_payments_v1_payments_proto),
		},
	},
	Contracts: []*pb.ContractDescriptor{
		paymentsModuleContract("payments.provider", "ProviderConfig"),
		paymentsStepContract("step.payment_charge", "PaymentChargeConfig", "PaymentChargeInput", "PaymentChargeOutput"),
		paymentsStepContract("step.payment_capture", "PaymentCaptureConfig", "PaymentCaptureInput", "PaymentCaptureOutput"),
		paymentsStepContract("step.payment_refund", "PaymentRefundConfig", "PaymentRefundInput", "PaymentRefundOutput"),
		paymentsStepContract("step.payment_fee_calculate", "PaymentFeeCalculateConfig", "PaymentFeeCalculateInput", "PaymentFeeCalculateOutput"),
		paymentsStepContract("step.payment_customer_ensure", "PaymentCustomerEnsureConfig", "PaymentCustomerEnsureInput", "PaymentCustomerEnsureOutput"),
		paymentsStepContract("step.payment_subscription_create", "PaymentSubscriptionCreateConfig", "PaymentSubscriptionCreateInput", "PaymentSubscriptionCreateOutput"),
		paymentsStepContract("step.payment_subscription_update", "PaymentSubscriptionUpdateConfig", "PaymentSubscriptionUpdateInput", "PaymentSubscriptionUpdateOutput"),
		paymentsStepContract("step.payment_subscription_cancel", "PaymentSubscriptionCancelConfig", "PaymentSubscriptionCancelInput", "PaymentSubscriptionCancelOutput"),
		paymentsStepContract("step.payment_checkout_create", "PaymentCheckoutCreateConfig", "PaymentCheckoutCreateInput", "PaymentCheckoutCreateOutput"),
		paymentsStepContract("step.payment_portal_create", "PaymentPortalCreateConfig", "PaymentPortalCreateInput", "PaymentPortalCreateOutput"),
		paymentsStepContract("step.payment_webhook_verify", "PaymentWebhookVerifyConfig", "PaymentWebhookVerifyInput", "PaymentWebhookVerifyOutput"),
		paymentsStepContract("step.payment_webhook_endpoint_ensure", "PaymentWebhookEndpointEnsureConfig", "PaymentWebhookEndpointEnsureInput", "PaymentWebhookEndpointEnsureOutput"),
		paymentsStepContract("step.payment_transfer", "PaymentTransferConfig", "PaymentTransferInput", "PaymentTransferOutput"),
		paymentsStepContract("step.payment_payout", "PaymentPayoutConfig", "PaymentPayoutInput", "PaymentPayoutOutput"),
		paymentsStepContract("step.payment_invoice_list", "PaymentInvoiceListConfig", "PaymentInvoiceListInput", "PaymentInvoiceListOutput"),
		paymentsStepContract("step.payment_method_attach", "PaymentMethodAttachConfig", "PaymentMethodAttachInput", "PaymentMethodAttachOutput"),
		paymentsStepContract("step.payment_method_list", "PaymentMethodListConfig", "PaymentMethodListInput", "PaymentMethodListOutput"),
	},
}

// paymentsProtoPackage is the fully-qualified protobuf package for this
// plugin's message types (matches the `package` directive in payments.proto).
const paymentsProtoPackage = "workflow.plugins.payments.v1."

func paymentsModuleContract(moduleType, configMessage string) *pb.ContractDescriptor {
	return &pb.ContractDescriptor{
		Kind:          pb.ContractKind_CONTRACT_KIND_MODULE,
		ModuleType:    moduleType,
		ConfigMessage: paymentsProtoPackage + configMessage,
		Mode:          pb.ContractMode_CONTRACT_MODE_STRICT_PROTO,
	}
}

func paymentsStepContract(stepType, configMessage, inputMessage, outputMessage string) *pb.ContractDescriptor {
	return &pb.ContractDescriptor{
		Kind:          pb.ContractKind_CONTRACT_KIND_STEP,
		StepType:      stepType,
		ConfigMessage: paymentsProtoPackage + configMessage,
		InputMessage:  paymentsProtoPackage + inputMessage,
		OutputMessage: paymentsProtoPackage + outputMessage,
		Mode:          pb.ContractMode_CONTRACT_MODE_STRICT_PROTO,
	}
}
