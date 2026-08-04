package linode_test

import (
	"context"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// Operation labels the cases below share with another table in this
// package. Naming each one once keeps the same string from being spelled
// in three places, which is what a plain method and its proto variant
// reporting under one label looks like.
const (
	labelGetAccount                 = "GetAccount"
	labelGetAccountChildAccount     = "GetAccountChildAccount"
	labelGetAccountEvent            = "GetAccountEvent"
	labelGetAccountOAuthClient      = "GetAccountOAuthClient"
	labelGetAccountPaymentMethod    = "GetAccountPaymentMethod"
	labelGetAccountServiceTransfer  = "GetAccountServiceTransfer"
	labelGetAccountSettings         = "GetAccountSettings"
	labelGetAccountUser             = "GetAccountUser"
	labelGetManagedCredential       = "GetManagedCredential"
	labelGetProfile                 = "GetProfile"
	labelGetProfileApp              = "GetProfileApp"
	labelUpdateOAuthClientThumbnail = "UpdateOAuthClientThumbnail"
)

// TestRoutedTransportAccountPart1 checks that each method below reports a failed
// connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportAccountPart1(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name: "AcceptAccountServiceTransfer",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.AcceptAccountServiceTransfer(ctx, "alpha"))
			},
		},
		{
			name: "AcknowledgeAccountAgreements",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.AcknowledgeAccountAgreements(ctx, &linode.AcknowledgeAccountAgreementsRequest{}))
			},
		},
		{
			name: "AddAccountPromoCredit",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.AddAccountPromoCredit(ctx, &linode.AddAccountPromoCreditRequest{}))
			},
		},
		{
			name: "AnswerProfileSecurityQuestions",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.AnswerProfileSecurityQuestions(ctx, &linode.AnswerProfileSecurityQuestionsRequest{}))
			},
		},
		{
			name:      "CancelAccountProto",
			operation: "CancelAccount",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CancelAccountProto(ctx, &linode.CancelAccountRequest{})

				return clientRouteError(err)
			},
		},
		{
			name: "ConfirmProfileTFAEnable",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ConfirmProfileTFAEnable(ctx, &linode.ProfileTFAEnableConfirmRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "CreateAccountChildAccountTokenProto",
			operation: "CreateAccountChildAccountToken",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateAccountChildAccountTokenProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      "CreateAccountPaymentMethodProto",
			operation: "CreateAccountPaymentMethod",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateAccountPaymentMethodProto(ctx, &linode.CreateAccountPaymentMethodRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "CreateAccountPaymentProto",
			operation: "CreateAccountPayment",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateAccountPaymentProto(ctx, &linode.CreateAccountPaymentRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "CreateAccountServiceTransferProto",
			operation: "CreateAccountServiceTransfer",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateAccountServiceTransferProto(ctx, &linode.CreateAccountServiceTransferRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "CreateAccountUserProto",
			operation: "CreateAccountUser",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateAccountUserProto(ctx, &linode.CreateAccountUserRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "CreateManagedCredentialProto",
			operation: "CreateManagedCredential",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateManagedCredentialProto(ctx, &linode.CreateManagedCredentialRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "CreateOAuthClientProto",
			operation: "CreateOAuthClient",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateOAuthClientProto(ctx, &linode.CreateOAuthClientRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "CreateProfileTokenProto",
			operation: "CreateProfileToken",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.CreateProfileTokenProto(ctx, linode.CreateProfileTokenRequest{})

				return clientRouteError(err)
			},
		},
		{
			name: "DeleteAccountOAuthClient",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteAccountOAuthClient(ctx, "alpha"))
			},
		},
		{
			name: "DeleteAccountPaymentMethod",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteAccountPaymentMethod(ctx, "alpha"))
			},
		},
		{
			name: "DeleteAccountServiceTransfer",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteAccountServiceTransfer(ctx, "alpha"))
			},
		},
		{
			name: "DeleteAccountUser",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteAccountUser(ctx, "alpha"))
			},
		},
		{
			name: "DeleteLongviewClient",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteLongviewClient(ctx, 4242))
			},
		},
		{
			name: "DeleteProfileApp",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteProfileApp(ctx, 4242))
			},
		},
		{
			name: "DeleteProfileDevice",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteProfileDevice(ctx, 4242))
			},
		},
		{
			name: "DeleteProfilePhoneNumber",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteProfilePhoneNumber(ctx))
			},
		},
		{
			name: "DeleteProfileToken",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DeleteProfileToken(ctx, 4242))
			},
		},
		{
			name: "DisableProfileTFA",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.DisableProfileTFA(ctx))
			},
		},
		{
			name:      "EnableProfileTFAProto",
			operation: "EnableProfileTFA",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.EnableProfileTFAProto(ctx)

				return clientRouteError(err)
			},
		},
		{
			name: "EnrollAccountBeta",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.EnrollAccountBeta(ctx, &linode.EnrollAccountBetaRequest{}))
			},
		},
		{
			name: labelGetAccount,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccount(ctx)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountAgreementsProto",
			operation: "GetAccountAgreements",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountAgreementsProto(ctx)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountAvailabilityProto",
			operation: "GetAccountAvailability",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountAvailabilityProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountBetaProto",
			operation: "GetAccountBeta",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountBetaProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
	})
}

// TestRoutedTransportAccountPart2 checks that each method below reports a failed
// connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportAccountPart2(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name: labelGetAccountChildAccount,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountChildAccount(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountChildAccountProto",
			operation: labelGetAccountChildAccount,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountChildAccountProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: labelGetAccountEvent,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountEvent(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountEventProto",
			operation: labelGetAccountEvent,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountEventProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountInvoiceProto",
			operation: "GetAccountInvoice",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountInvoiceProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountLoginProto",
			operation: "GetAccountLogin",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountLoginProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: labelGetAccountOAuthClient,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountOAuthClient(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountOAuthClientProto",
			operation: labelGetAccountOAuthClient,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountOAuthClientProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: labelGetAccountPaymentMethod,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountPaymentMethod(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountPaymentMethodProto",
			operation: labelGetAccountPaymentMethod,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountPaymentMethodProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountPaymentProto",
			operation: "GetAccountPayment",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountPaymentProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountProto",
			operation: labelGetAccount,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountProto(ctx)

				return clientRouteError(err)
			},
		},
		{
			name: labelGetAccountServiceTransfer,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountServiceTransfer(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountServiceTransferProto",
			operation: labelGetAccountServiceTransfer,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountServiceTransferProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: labelGetAccountSettings,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountSettings(ctx)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountSettingsProto",
			operation: labelGetAccountSettings,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountSettingsProto(ctx)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountTransferProto",
			operation: "GetAccountTransfer",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountTransferProto(ctx)

				return clientRouteError(err)
			},
		},
		{
			name: labelGetAccountUser,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountUser(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: "GetAccountUserGrants",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountUserGrants(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountUserGrantsProto",
			operation: "GetAccountUserGrants",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountUserGrantsProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      "GetAccountUserProto",
			operation: labelGetAccountUser,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetAccountUserProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name:      "GetBetaProto",
			operation: "GetBeta",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetBetaProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: labelGetManagedCredential,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetManagedCredential(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetManagedCredentialProto",
			operation: labelGetManagedCredential,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetManagedCredentialProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetManagedSSHKeyProto",
			operation: "GetManagedSSHKey",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetManagedSSHKeyProto(ctx)

				return clientRouteError(err)
			},
		},
		{
			name: "GetOAuthClientThumbnail",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetOAuthClientThumbnail(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: labelGetProfileApp,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetProfileApp(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetProfileAppProto",
			operation: labelGetProfileApp,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetProfileAppProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: "GetProfileDevice",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetProfileDevice(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetProfileDeviceProto",
			operation: "GetProfileDevice",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetProfileDeviceProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
	})
}

// TestRoutedTransportAccountPart3 checks that each method below reports a failed
// connection as a *linode.NetworkError naming its own operation.
func TestRoutedTransportAccountPart3(t *testing.T) {
	t.Parallel()

	runRoutedTransportCases(t, []routedTransportCase{
		{
			name:      "GetProfileLoginProto",
			operation: "GetProfileLogin",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetProfileLoginProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: "GetProfilePreferences",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetProfilePreferences(ctx)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetProfileProto",
			operation: labelGetProfile,
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetProfileProto(ctx)

				return clientRouteError(err)
			},
		},
		{
			name:      "GetProfileTokenProto",
			operation: "GetProfileToken",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.GetProfileTokenProto(ctx, 4242)

				return clientRouteError(err)
			},
		},
		{
			name: "MakeAccountPaymentMethodDefault",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.MakeAccountPaymentMethodDefault(ctx, "alpha"))
			},
		},
		{
			name: "MarkAccountEventSeen",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.MarkAccountEventSeen(ctx, 4242))
			},
		},
		{
			name:      "ResetOAuthClientSecretProto",
			operation: "ResetOAuthClientSecret",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.ResetOAuthClientSecretProto(ctx, "alpha")

				return clientRouteError(err)
			},
		},
		{
			name: "RevokeManagedCredential",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.RevokeManagedCredential(ctx, 4242))
			},
		},
		{
			name: "SendProfilePhoneNumberVerificationCode",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.SendProfilePhoneNumberVerificationCode(ctx, &linode.ProfilePhoneNumberRequest{}))
			},
		},
		{
			name:      "UpdateAccountProto",
			operation: "UpdateAccount",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateAccountProto(ctx, &linode.UpdateAccountRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateAccountSettingsProto",
			operation: "UpdateAccountSettings",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateAccountSettingsProto(ctx, &linode.UpdateAccountSettingsRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateAccountUserGrantsProto",
			operation: "UpdateAccountUserGrants",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateAccountUserGrantsProto(ctx, "alpha", &linode.UpdateAccountUserGrantsRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateAccountUserProto",
			operation: "UpdateAccountUser",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateAccountUserProto(ctx, "alpha", &linode.UpdateAccountUserRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateLongviewClientProto",
			operation: "UpdateLongviewClient",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateLongviewClientProto(ctx, 4242, &linode.UpdateLongviewClientRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateManagedCredentialProto",
			operation: "UpdateManagedCredential",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateManagedCredentialProto(ctx, 4242, linode.UpdateManagedCredentialRequest{})

				return clientRouteError(err)
			},
		},
		{
			name:      "UpdateOAuthClientProto",
			operation: "UpdateOAuthClient",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateOAuthClientProto(ctx, "alpha", &linode.UpdateOAuthClientRequest{})

				return clientRouteError(err)
			},
		},
		{
			name: labelUpdateOAuthClientThumbnail,
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.UpdateOAuthClientThumbnail(ctx, "alpha", []byte("png")))
			},
		},
		{
			name:      "UpdateProfileTokenProto",
			operation: "UpdateProfileToken",
			call: func(ctx context.Context, client *linode.Client) error {
				_, err := client.UpdateProfileTokenProto(ctx, "alpha", linode.UpdateProfileTokenRequest{})

				return clientRouteError(err)
			},
		},
		{
			name: "VerifyProfilePhoneNumber",
			call: func(ctx context.Context, client *linode.Client) error {
				return clientRouteError(client.VerifyProfilePhoneNumber(ctx, &linode.ProfilePhoneNumberVerifyRequest{}))
			},
		},
	})
}
