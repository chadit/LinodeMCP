package toolhooks_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/toolhooks"
)

const (
	keyTransferLinodeIDs = "linode_ids"
	keyTransferEntities  = "entities"
)

// TestLinodeAccountServiceTransferCreateNormalize: entities is the only shape
// the route accepts, so the convenience form has to become one before the body
// is built.
func TestLinodeAccountServiceTransferCreateNormalize(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args        map[string]any
		wantLinodes []int
		wantFolded  bool
	}{
		"ids fold into entities": {
			args:        map[string]any{keyTransferLinodeIDs: []any{float64(7), float64(9)}},
			wantFolded:  true,
			wantLinodes: []int{7, 9},
		},
		"a caller-supplied entities object wins": {
			args: map[string]any{
				keyTransferLinodeIDs: []any{float64(7)},
				keyTransferEntities:  map[string]any{"domains": []any{float64(9)}},
			},
		},
		"an empty entities object falls through to the ids": {
			args:        map[string]any{keyTransferLinodeIDs: []any{float64(7)}, keyTransferEntities: map[string]any{}},
			wantFolded:  true,
			wantLinodes: []int{7},
		},
		"absent ids stay absent":        {args: map[string]any{}},
		"an unusable ids value is left": {args: map[string]any{keyTransferLinodeIDs: "seven"}},
		"an empty ids list is left":     {args: map[string]any{keyTransferLinodeIDs: []any{}}},
		"a non-object entities value is left": {
			args: map[string]any{keyTransferLinodeIDs: []any{float64(7)}, keyTransferEntities: []any{1}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := requestWith(test.args)
			_, suppliedIDs := request.GetArguments()[keyTransferLinodeIDs]

			toolhooks.LinodeAccountServiceTransferCreateNormalize(&request)

			arguments := request.GetArguments()
			wantPresent := suppliedIDs && !test.wantFolded

			if _, present := arguments[keyTransferLinodeIDs]; present != wantPresent {
				t.Errorf("linode_ids present = %v, want %v", present, wantPresent)
			}

			if !test.wantFolded {
				return
			}

			entities, isObject := arguments[keyTransferEntities].(map[string]any)
			if !isObject {
				t.Fatalf("entities = %v, want an object", arguments[keyTransferEntities])
			}

			linodes, isIDs := entities["linodes"].([]int)
			if !isIDs || len(linodes) != len(test.wantLinodes) {
				t.Fatalf("entities.linodes = %v, want %v", entities["linodes"], test.wantLinodes)
			}

			for index, want := range test.wantLinodes {
				if linodes[index] != want {
					t.Errorf("entities.linodes[%d] = %d, want %d", index, linodes[index], want)
				}
			}
		})
	}
}
