package linode_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

// TestRegenerateLKEClusterRequestPayload pins that a regenerate selecting
// neither credential sends no body at all, which is what the endpoint accepted
// before the flags existed.
func TestRegenerateLKEClusterRequestPayload(t *testing.T) {
	t.Parallel()

	if payload := (linode.RegenerateLKEClusterRequest{}).Payload(); payload != nil {
		t.Errorf("Payload() = %v, want nil", payload)
	}

	req := linode.RegenerateLKEClusterRequest{ServiceToken: true}
	if payload := req.Payload(); payload != req {
		t.Errorf("Payload() = %v, want %v", payload, req)
	}
}
