package linode_test

import (
	"context"
	"net/http"
	"testing"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestClientRoutesAccountPart1 pins the account client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesAccountPart1(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "AcceptAccountServiceTransfer",
			wantVerb: http.MethodPost,
			wantPath: "/account/service-transfers/alpha/accept",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.AcceptAccountServiceTransfer(ctx, "alpha"))
			},
		},
		{
			name:     "AcknowledgeAccountAgreements",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathAccountAgreements,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.AcknowledgeAccountAgreements(ctx, &linode.AcknowledgeAccountAgreementsRequest{}))
			},
		},
		{
			name:     "AddAccountPromoCredit",
			wantVerb: http.MethodPost,
			wantPath: "/account/promo-codes",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.AddAccountPromoCredit(ctx, &linode.AddAccountPromoCreditRequest{}))
			},
		},
		{
			name:     "AnswerProfileSecurityQuestions",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathProfileSecurityQuestions,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.AnswerProfileSecurityQuestions(ctx, &linode.AnswerProfileSecurityQuestionsRequest{}))
			},
		},
		{
			name:     "CancelAccountProto",
			wantVerb: http.MethodPost,
			wantPath: "/account/cancel",
			response: clientRouteProtoObjSurveyLink,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CancelAccountProto(ctx, &linode.CancelAccountRequest{})

				return clientRouteProbe(err, func() any { return got.GetSurveyLink() })
			},
		},
		{
			name:     "CreateAccountChildAccountTokenProto",
			wantVerb: http.MethodPost,
			wantPath: "/account/child-accounts/alpha/token",
			response: clientRouteProtoObjCreated,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateAccountChildAccountTokenProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetCreated() })
			},
		},
		{
			name:     "CreateAccountServiceTransferProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathAccountServiceTransfers,
			response: clientRouteProtoObjCreated,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateAccountServiceTransferProto(ctx, &linode.CreateAccountServiceTransferRequest{})

				return clientRouteProbe(err, func() any { return got.GetCreated() })
			},
		},
		{
			name:     "CreateAccountUserProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathAccountUsers,
			response: clientRouteProtoObjEmail,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateAccountUserProto(ctx, &linode.CreateAccountUserRequest{})

				return clientRouteProbe(err, func() any { return got.GetEmail() })
			},
		},
		{
			name:     "CreateManagedCredentialProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathManagedCredentials,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateManagedCredentialProto(ctx, &linode.CreateManagedCredentialRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateOAuthClientProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathAccountOauthClients,
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateOAuthClientProto(ctx, &linode.CreateOAuthClientRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "CreateProfileTokenProto",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathProfileTokens,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.CreateProfileTokenProto(ctx, linode.CreateProfileTokenRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "DeleteAccountOAuthClient",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathAccountOauthClientsAlpha,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteAccountOAuthClient(ctx, "alpha"))
			},
		},
	})
}

// TestClientRoutesAccountPart2 pins the account client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesAccountPart2(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "DeleteAccountPaymentMethod",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathAccountPaymentMethodsAlpha,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteAccountPaymentMethod(ctx, "alpha"))
			},
		},
		{
			name:     "DeleteAccountServiceTransfer",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathAccountServiceTransfersAlpha,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteAccountServiceTransfer(ctx, "alpha"))
			},
		},
		{
			name:     "DeleteAccountUser",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathAccountUsersAlpha,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteAccountUser(ctx, "alpha"))
			},
		},
		{
			name:     "DeleteLongviewClient",
			wantVerb: http.MethodDelete,
			wantPath: "/longview/clients/4242",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteLongviewClient(ctx, 4242))
			},
		},
		{
			name:     "DeleteProfileApp",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathProfileApps4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteProfileApp(ctx, 4242))
			},
		},
		{
			name:     "DeleteProfileDevice",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathProfileDevices4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteProfileDevice(ctx, 4242))
			},
		},
		{
			name:     "DeleteProfilePhoneNumber",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathProfilePhoneNumber,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteProfilePhoneNumber(ctx))
			},
		},
		{
			name:     "DeleteProfileToken",
			wantVerb: http.MethodDelete,
			wantPath: clientRoutePathProfileTokens4242,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DeleteProfileToken(ctx, 4242))
			},
		},
		{
			name:     "DisableProfileTFA",
			wantVerb: http.MethodPost,
			wantPath: "/profile/tfa-disable",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.DisableProfileTFA(ctx))
			},
		},
		{
			name:     "EnableAccountManaged",
			wantVerb: http.MethodPost,
			wantPath: "/account/settings/managed-enable",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.EnableAccountManaged(ctx))
			},
		},
		{
			name:     "EnableProfileTFAProto",
			wantVerb: http.MethodPost,
			wantPath: "/profile/tfa-enable",
			response: clientRouteProtoObjSecret,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.EnableProfileTFAProto(ctx)

				return clientRouteProbe(err, func() any { return got.GetSecret() })
			},
		},
		{
			name:     "EnrollAccountBeta",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathAccountBetas,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.EnrollAccountBeta(ctx, &linode.EnrollAccountBetaRequest{}))
			},
		},
	})
}

// TestClientRoutesAccountPart3 pins the account client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesAccountPart3(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetAccount",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccount,
			response: clientRouteObjZip,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccount(ctx)

				return clientRouteProbe(err, func() any { return got.Zip })
			},
		},
		{
			name:     "GetAccountAgreementsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountAgreements,
			response: clientRouteProtoObjBillingAgreementBool,
			want:     true,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountAgreementsProto(ctx)

				return clientRouteProbe(err, func() any { return got.GetBillingAgreement() })
			},
		},
		{
			name:     "GetAccountAvailabilityProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/availability/alpha",
			response: clientRouteProtoObjRegion,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountAvailabilityProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetRegion() })
			},
		},
		{
			name:     "GetAccountBetaProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/betas/alpha",
			response: clientRouteProtoObjEnrolled,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountBetaProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetEnrolled() })
			},
		},
		{
			name:     "GetAccountChildAccount",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountChildAccountsAlpha,
			response: clientRouteObjEuuid,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountChildAccount(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.EUUID })
			},
		},
		{
			name:     "GetAccountChildAccountProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountChildAccountsAlpha,
			response: clientRouteProtoObjActiveSince,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountChildAccountProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetActiveSince() })
			},
		},
		{
			name:     "GetAccountEvent",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountEvents4242,
			response: clientRouteObjCreated,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountEvent(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Created })
			},
		},
		{
			name:     "GetAccountEventProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountEvents4242,
			response: clientRouteProtoObjAction,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountEventProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetAction() })
			},
		},
		{
			name:     "GetAccountInvoiceProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/invoices/4242",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountInvoiceProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetAccountLoginProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/logins/4242",
			response: clientRouteProtoObjDatetime,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountLoginProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetDatetime() })
			},
		},
		{
			name:     "GetAccountOAuthClient",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountOauthClientsAlpha,
			response: clientRouteObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountOAuthClient(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.ID })
			},
		},
		{
			name:     "GetAccountOAuthClientProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountOauthClientsAlpha,
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountOAuthClientProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
	})
}

// TestClientRoutesAccountPart4 pins the account client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesAccountPart4(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetAccountPaymentMethod",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountPaymentMethodsAlpha,
			response: clientRouteObjType,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountPaymentMethod(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.Type })
			},
		},
		{
			name:     "GetAccountPaymentMethodProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountPaymentMethodsAlpha,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountPaymentMethodProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetAccountPaymentProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/payments/4242",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountPaymentProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetAccountProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccount,
			response: clientRouteProtoObjFirstName,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountProto(ctx)

				return clientRouteProbe(err, func() any { return got.GetFirstName() })
			},
		},
		{
			name:     "GetAccountServiceTransfer",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountServiceTransfersAlpha,
			response: clientRouteObjCreated,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountServiceTransfer(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.Created })
			},
		},
		{
			name:     "GetAccountServiceTransferProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountServiceTransfersAlpha,
			response: clientRouteProtoObjCreated,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountServiceTransferProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetCreated() })
			},
		},
		{
			name:     "GetAccountSettings",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountSettings,
			response: clientRouteObjInterfacesForNewLinodes,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountSettings(ctx)

				return clientRouteProbe(err, func() any { return got.InterfacesForNewLinodes })
			},
		},
		{
			name:     "GetAccountSettingsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountSettings,
			response: clientRouteProtoObjBackupsEnabledBool,
			want:     true,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountSettingsProto(ctx)

				return clientRouteProbe(err, func() any { return got.GetBackupsEnabled() })
			},
		},
		{
			name:     "GetAccountTransferProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/transfer",
			response: clientRouteProtoObjBillableInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountTransferProto(ctx)

				return clientRouteProbe(err, func() any { return got.GetBillable() })
			},
		},
		{
			name:     "GetAccountUser",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountUsersAlpha,
			response: clientRouteObjEmail,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountUser(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.Email })
			},
		},
		{
			name:     "GetAccountUserProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountUsersAlpha,
			response: clientRouteProtoObjEmail,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetAccountUserProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetEmail() })
			},
		},
		{
			name:     "GetBetaProto",
			wantVerb: http.MethodGet,
			wantPath: "/betas/alpha",
			response: clientRouteProtoObjClass,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetBetaProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetClass() })
			},
		},
	})
}

// TestClientRoutesAccountPart5 pins the account client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesAccountPart5(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "GetManagedCredential",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedCredentials4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedCredential(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetManagedCredentialProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedCredentials4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedCredentialProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetManagedSSHKeyProto",
			wantVerb: http.MethodGet,
			wantPath: "/managed/credentials/sshkey",
			response: clientRouteProtoObjSSHKey,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetManagedSSHKeyProto(ctx)

				return clientRouteProbe(err, func() any { return got.GetSshKey() })
			},
		},
		{
			name:     "GetProfile",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfile,
			response: clientRouteObjUsername,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetProfile(ctx)

				return clientRouteProbe(err, func() any { return got.Username })
			},
		},
		{
			name:     "GetProfileApp",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfileApps4242,
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetProfileApp(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "GetProfileAppProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfileApps4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetProfileAppProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetProfileDeviceProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfileDevices4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetProfileDeviceProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "GetProfileLoginProto",
			wantVerb: http.MethodGet,
			wantPath: "/profile/logins/4242",
			response: clientRouteProtoObjDatetime,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetProfileLoginProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetDatetime() })
			},
		},
		{
			name:     "GetProfileProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfile,
			response: clientRouteProtoObjUsername,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetProfileProto(ctx)

				return clientRouteProbe(err, func() any { return got.GetUsername() })
			},
		},
		{
			name:     "GetProfileTokenProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfileTokens4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.GetProfileTokenProto(ctx, 4242)

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "ListAccountAvailabilityProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/availability",
			response: clientRouteProtoPageRegion,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAccountAvailabilityProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.AccountAvailability).GetRegion) })
			},
		},
		{
			name:     "ListAccountBetasProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountBetas,
			response: clientRouteProtoPageEnrolled,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAccountBetasProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.AccountBetaProgram).GetEnrolled) })
			},
		},
	})
}

// TestClientRoutesAccountPart6 pins the account client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesAccountPart6(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "ListAccountChildAccountsProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/child-accounts",
			response: clientRouteProtoPageActiveSince,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAccountChildAccountsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ChildAccount).GetActiveSince) })
			},
		},
		{
			name:     "ListAccountEventsProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/events",
			response: clientRouteProtoPageAction,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAccountEventsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.AccountEvent).GetAction) })
			},
		},
		{
			name:     "ListAccountInvoiceItemsProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/invoices/4242/items",
			response: clientRouteProtoPageFrom,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAccountInvoiceItemsProto(ctx, 4242, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.AccountInvoiceItem).GetFrom) })
			},
		},
		{
			name:     "ListAccountInvoicesProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/invoices",
			response: clientRouteProtoPageDate,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAccountInvoicesProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.AccountInvoice).GetDate) })
			},
		},
		{
			name:     "ListAccountLoginsProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/logins",
			response: clientRouteProtoPageDatetime,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAccountLoginsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.AccountLogin).GetDatetime) })
			},
		},
		{
			name:     "ListAccountMaintenanceProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/maintenance",
			response: clientRouteProtoPageReason,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAccountMaintenanceProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.AccountMaintenance).GetReason) })
			},
		},
		{
			name:     "ListAccountNotificationsProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/notifications",
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAccountNotificationsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.AccountNotification).GetLabel) })
			},
		},
		{
			name:     "ListAccountOAuthClientsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountOauthClients,
			response: clientRouteProtoPageID,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAccountOAuthClientsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.OAuthClient).GetId) })
			},
		},
		{
			name:     "ListAccountPaymentMethodsProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/payment-methods",
			response: clientRouteProtoPageType,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAccountPaymentMethodsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.AccountPaymentMethod).GetType) })
			},
		},
		{
			name:     "ListAccountPaymentsProto",
			wantVerb: http.MethodGet,
			wantPath: "/account/payments",
			response: clientRouteProtoPageDate,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAccountPaymentsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.AccountPayment).GetDate) })
			},
		},
		{
			name:     "ListAccountServiceTransfersProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountServiceTransfers,
			response: clientRouteProtoPageCreated,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAccountServiceTransfersProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.AccountEntityTransfer).GetCreated) })
			},
		},
		{
			name:     "ListAccountUsersProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathAccountUsers,
			response: clientRouteProtoPageEmail,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListAccountUsersProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.AccountUser).GetEmail) })
			},
		},
	})
}

// TestClientRoutesAccountPart7 pins the account client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesAccountPart7(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "ListBetasProto",
			wantVerb: http.MethodGet,
			wantPath: "/betas",
			response: clientRouteProtoPageClass,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListBetasProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.BetaProgram).GetClass) })
			},
		},
		{
			name:     "ListLongviewClientsProto",
			wantVerb: http.MethodGet,
			wantPath: "/longview/clients",
			response: clientRouteProtoPageCreated,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListLongviewClientsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.LongviewClient).GetCreated) })
			},
		},
		{
			name:     "ListMaintenancePoliciesProto",
			wantVerb: http.MethodGet,
			wantPath: "/maintenance/policies",
			response: clientRouteProtoPageSlug,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListMaintenancePoliciesProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.MaintenancePolicy).GetSlug) })
			},
		},
		{
			name:     "ListManagedCredentialsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathManagedCredentials,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListManagedCredentialsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ManagedCredential).GetLabel) })
			},
		},
		{
			name:     "ListProfileAppsProto",
			wantVerb: http.MethodGet,
			wantPath: "/profile/apps",
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListProfileAppsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.ProfileApp).GetLabel) })
			},
		},
		{
			name:     "ListProfileDevicesProto",
			wantVerb: http.MethodGet,
			wantPath: "/profile/devices",
			response: clientRouteProtoPageCreated,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListProfileDevicesProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.TrustedDevice).GetCreated) })
			},
		},
		{
			name:     "ListProfileLoginsProto",
			wantVerb: http.MethodGet,
			wantPath: "/profile/logins",
			response: clientRouteProtoPageDatetime,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListProfileLoginsProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.AccountLogin).GetDatetime) })
			},
		},
		{
			name:     "ListProfileSecurityQuestionsProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfileSecurityQuestions,
			response: clientRouteSecurityQuestionsEnvelope,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListProfileSecurityQuestionsProto(ctx)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.SecurityQuestion).GetQuestion) })
			},
		},
		{
			name:     "ListProfileTokensProto",
			wantVerb: http.MethodGet,
			wantPath: clientRoutePathProfileTokens,
			response: clientRouteProtoPageLabel,
			want:     clientRouteTwoElementProbe,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ListProfileTokensProto(ctx, 1, 25)

				return clientRouteProbe(err, func() any { return clientRouteList(got, (*linodev1.PersonalAccessToken).GetLabel) })
			},
		},
		{
			name:     "MakeAccountPaymentMethodDefault",
			wantVerb: http.MethodPost,
			wantPath: "/account/payment-methods/alpha/make-default",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.MakeAccountPaymentMethodDefault(ctx, "alpha"))
			},
		},
		{
			name:     "MarkAccountEventSeen",
			wantVerb: http.MethodPost,
			wantPath: "/account/events/4242/seen",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.MarkAccountEventSeen(ctx, 4242))
			},
		},
		{
			name:     "ResetOAuthClientSecretProto",
			wantVerb: http.MethodPost,
			wantPath: "/account/oauth-clients/alpha/reset-secret",
			response: clientRouteProtoObjSecret,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.ResetOAuthClientSecretProto(ctx, "alpha")

				return clientRouteProbe(err, func() any { return got.GetSecret() })
			},
		},
	})
}

// TestClientRoutesAccountPart8 pins the account client methods listed below to the
// request each one issues and the value it decodes back.
func TestClientRoutesAccountPart8(t *testing.T) {
	t.Parallel()

	runClientRouteCases(t, []clientRouteCase{
		{
			name:     "RevokeManagedCredential",
			wantVerb: http.MethodPost,
			wantPath: "/managed/credentials/4242/revoke",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.RevokeManagedCredential(ctx, 4242))
			},
		},
		{
			name:     "SendProfilePhoneNumberVerificationCode",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathProfilePhoneNumber,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.SendProfilePhoneNumberVerificationCode(ctx, &linode.ProfilePhoneNumberRequest{}))
			},
		},
		{
			name:     "UpdateAccountProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathAccount,
			response: clientRouteProtoObjFirstName,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateAccountProto(ctx, &linode.UpdateAccountRequest{})

				return clientRouteProbe(err, func() any { return got.GetFirstName() })
			},
		},
		{
			name:     "UpdateAccountSettingsProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathAccountSettings,
			response: clientRouteProtoObjBackupsEnabledBool,
			want:     true,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateAccountSettingsProto(ctx, &linode.UpdateAccountSettingsRequest{})

				return clientRouteProbe(err, func() any { return got.GetBackupsEnabled() })
			},
		},
		{
			name:     "UpdateAccountUserProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathAccountUsersAlpha,
			response: clientRouteProtoObjEmail,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateAccountUserProto(ctx, "alpha", &linode.UpdateAccountUserRequest{})

				return clientRouteProbe(err, func() any { return got.GetEmail() })
			},
		},
		{
			name:     "UpdateManagedCredentialProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathManagedCredentials4242,
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateManagedCredentialProto(ctx, 4242, linode.UpdateManagedCredentialRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateManagedCredentialUsernamePassword",
			wantVerb: http.MethodPost,
			wantPath: "/managed/credentials/4242/update",
			response: clientRouteObjLabel,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateManagedCredentialUsernamePassword(ctx, 4242, &linode.UpdateManagedCredentialUsernamePasswordRequest{})

				return clientRouteProbe(err, func() any { return got.Label })
			},
		},
		{
			name:     "UpdateOAuthClientProto",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathAccountOauthClientsAlpha,
			response: clientRouteProtoObjID,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateOAuthClientProto(ctx, "alpha", &linode.UpdateOAuthClientRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "UpdateOAuthClientThumbnail",
			wantVerb: http.MethodPut,
			wantPath: "/account/oauth-clients/alpha/thumbnail",
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.UpdateOAuthClientThumbnail(ctx, "alpha", []byte("payload")))
			},
		},
		{
			name:     "UpdateProfile",
			wantVerb: http.MethodPut,
			wantPath: clientRoutePathProfile,
			response: clientRouteObjUsername,
			want:     clientRouteProbeValue,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateProfile(ctx, &linode.UpdateProfileRequest{})

				return clientRouteProbe(err, func() any { return got.Username })
			},
		},
		{
			name:     "UpdateProfileTokenProto",
			wantVerb: http.MethodPut,
			wantPath: "/profile/tokens/alpha",
			response: clientRouteProtoObjIDInt32,
			want:     int32(4242),
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				got, err := client.UpdateProfileTokenProto(ctx, "alpha", linode.UpdateProfileTokenRequest{})

				return clientRouteProbe(err, func() any { return got.GetId() })
			},
		},
		{
			name:     "VerifyProfilePhoneNumber",
			wantVerb: http.MethodPost,
			wantPath: clientRoutePathProfilePhoneNumberVerify,
			response: clientRouteEmptyObject,
			call: func(ctx context.Context, client *linode.Client) (any, error) {
				return nil, clientRouteError(client.VerifyProfilePhoneNumber(ctx, &linode.ProfilePhoneNumberVerifyRequest{}))
			},
		},
	})
}
