package toolhooks_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	accountUserName      = "newuser"
	accountUserEmail     = "ops@example.com"
	accountUsersRoute    = "/account/users"
	keyAccountUserEmail  = "email"
	keyAccountRestricted = "restricted"
)

// TestLinodeAccountUserCreatePreviewNamesTheUser: the request bytes say which
// user is added but not that adding one mails an invite, and Go reported no
// sentence at all here before the hook.
func TestLinodeAccountUserCreatePreviewNamesTheUser(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		restricted any
		want       string
	}{
		"restricted omitted": {
			restricted: nil,
			want:       `A new account user "newuser" will be created with restricted=false.`,
		},
		"restricted true": {
			restricted: true,
			want:       `A new account user "newuser" will be created with restricted=true.`,
		},
		"restricted false": {
			restricted: false,
			want:       `A new account user "newuser" will be created with restricted=false.`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			arguments := map[string]any{
				argUsername:         accountUserName,
				keyAccountUserEmail: accountUserEmail,
				keyDryRun:           true,
			}
			if test.restricted != nil {
				arguments[keyAccountRestricted] = test.restricted
			}

			request := requestWith(arguments)

			result, err := toolhooks.LinodeAccountUserCreatePreview(
				t.Context(), &request, configFor(""), http.MethodPost, accountUsersRoute,
				map[string]any{argUsername: accountUserName, keyAccountUserEmail: accountUserEmail},
			)
			if err != nil {
				t.Fatalf("LinodeAccountUserCreatePreview: %v", err)
			}

			var preview map[string]any
			if err := json.Unmarshal([]byte(resultText(t, result)), &preview); err != nil {
				t.Fatalf("decode preview: %v", err)
			}

			if preview["tool"] != "linode_account_user_create" {
				t.Errorf("tool = %v, want linode_account_user_create", preview["tool"])
			}

			effects, _ := preview["side_effects"].([]any)
			if len(effects) != 1 || effects[0] != test.want {
				t.Errorf("side_effects = %v, want [%q]", preview["side_effects"], test.want)
			}

			if preview["current_state"] != nil {
				t.Errorf("current_state = %v, want nil: the user does not exist yet", preview["current_state"])
			}

			would, _ := preview["would_execute"].(map[string]any)
			if would["method"] != http.MethodPost || would["path"] != accountUsersRoute {
				t.Errorf("would_execute = %v %v, want POST %s", would["method"], would["path"], accountUsersRoute)
			}

			body, _ := would["body"].(map[string]any)
			if body[argUsername] != accountUserName || body[keyAccountUserEmail] != accountUserEmail {
				t.Errorf("would_execute.body = %v, want the username and email echoed", would["body"])
			}
		})
	}
}
