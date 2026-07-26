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

// TestMarshalReservedIPResponseProtoMarshalError covers the single-address
// twin of the list path's marshal failure. A valid API object cannot reach it,
// so the marshaller is injected the same way.
func TestMarshalReservedIPResponseProtoMarshalError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("marshal failed")

	result, err := marshalReservedIPResponseWithMarshal(
		json.RawMessage(`{"address":"192.0.2.10"}`),
		func(proto.Message) ([]byte, error) { return nil, sentinel },
	)
	if result != nil || !errors.Is(err, sentinel) {
		t.Errorf("result = %v, err = %v, want the marshal error", result, err)
	}
}

// TestMarshalReservedIPResponseDecodeError proves a body that is not an object
// fails loudly rather than rendering as an address with every field empty.
func TestMarshalReservedIPResponseDecodeError(t *testing.T) {
	t.Parallel()

	result, err := marshalReservedIPResponse(json.RawMessage(`[]`))
	if result != nil || err == nil {
		t.Errorf("result = %v, err = %v, want a decode error", result, err)
	}
}

// TestMarshalReservedIPJSONError covers the render branch with a field holding
// invalid JSON, which no decoded response produces.
func TestMarshalReservedIPJSONError(t *testing.T) {
	t.Parallel()

	result, err := marshalReservedIPJSON(&reservedIPAddressJSON{
		Address: json.RawMessage(`{"address":`),
	})
	if result != nil || err == nil {
		t.Errorf("result = %v, err = %v, want a marshal error", result, err)
	}
}
