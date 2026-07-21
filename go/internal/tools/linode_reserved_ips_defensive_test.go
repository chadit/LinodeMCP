package tools //nolint:externaltestpkg // Private defensive branches cannot be reached through valid API responses.

import (
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestMarshalReservedIPListResponseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		page   *linode.ReservedIPListPage
		wantIs error
	}{
		{
			name:   "typed and raw item counts differ",
			page:   &linode.ReservedIPListPage{ReservedIPs: []*linodev1.ReservedIPAddress{{}}},
			wantIs: errReservedIPListShape,
		},
		{
			name: "proto item cannot be marshaled",
			page: &linode.ReservedIPListPage{
				ReservedIPs:    []*linodev1.ReservedIPAddress{{Address: string([]byte{0xff})}},
				RawReservedIPs: []json.RawMessage{json.RawMessage(`{}`)},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := marshalReservedIPListResponse(testCase.page)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			if testCase.wantIs != nil && !errors.Is(err, testCase.wantIs) {
				t.Fatalf("error = %v, want wrapped %v", err, testCase.wantIs)
			}

			if result != nil {
				t.Errorf("result = %+v, want nil", result)
			}
		})
	}
}

func TestReservedIPAddressResponseRawDecodeError(t *testing.T) {
	t.Parallel()

	_, err := reservedIPAddressResponse(
		&linodev1.ReservedIPAddress{},
		json.RawMessage(`{"address":`),
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestMarshalReservedIPListJSONError(t *testing.T) {
	t.Parallel()

	result, err := marshalReservedIPListJSON(reservedIPListJSON{
		ReservedIPs: []reservedIPAddressJSON{{Address: json.RawMessage(`{"address":`)}},
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %+v, want nil", result)
	}
}

func TestReservedIPAddressResponseProtoDecodeError(t *testing.T) {
	t.Parallel()

	_, err := reservedIPAddressResponseWithMarshal(
		&linodev1.ReservedIPAddress{},
		json.RawMessage(`{}`),
		func(proto.Message) ([]byte, error) {
			return []byte(`{"address":`), nil
		},
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}
