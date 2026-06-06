package tools_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const imageSGGetPath = "/images/sharegroups/123"

func TestLinodeImageUploadToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageUploadTool(&config.Config{})
		assertContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageUploadTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  "my-image",
			keyRegion: regionUSEast,
			keyDryRun: true,
		}))
		requireNoError(t, err)
		requireFalse(t, result.IsError)

		var body map[string]any
		requireNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assertEqual(t, "linode_image_upload", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assertEqual(t, "POST", would["method"])
		assertEqual(t, "/images/upload", would["path"])
		assertNil(t, body["current_state"], "create has no existing resource to preview")
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageUploadTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast,
			keyDryRun: true,
		}))
		requireNoError(t, err)
		assertTrue(t, result.IsError)
		assertErrorContains(t, result, "label is required")
	})
}

func TestLinodeImageCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageCreateTool(&config.Config{})
		assertContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDiskID: float64(456),
			keyDryRun: true,
		}))
		requireNoError(t, err)
		requireFalse(t, result.IsError)

		var body map[string]any
		requireNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assertEqual(t, "linode_image_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assertEqual(t, "POST", would["method"])
		assertEqual(t, "/images", would["path"])
		assertNil(t, body["current_state"], "create has no existing resource to preview")

		sideEffects, _ := body["side_effects"].([]any)
		requireLen(t, sideEffects, 1, "create surfaces the image-capture side effect")

		effect, gotString := sideEffects[0].(string)
		requireTrue(t, gotString)
		assertContains(t, effect, "456", "side effect should name the source disk")
	})

	t.Run("still validates disk_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		requireNoError(t, err)
		assertTrue(t, result.IsError)
	})
}

func TestLinodeImageReplicateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageReplicateTool(&config.Config{})
		assertContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without replicating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/images/"+privateImage12345Fixture,
			linode.Image{ID: privateImage12345Fixture})
		_, _, handler := tools.NewLinodeImageReplicateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyImageID: privateImage12345Fixture,
			keyRegions: `["us-east"]`,
			keyDryRun:  true,
		}))
		requireNoError(t, err)
		requireFalse(t, result.IsError)

		var body map[string]any
		requireNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assertEqual(t, "linode_image_replicate", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assertEqual(t, "POST", would["method"])
		assertEqual(t, "/images/"+privateImage12345Fixture+"/regions", would["path"])
		assertEqual(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates regions", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageReplicateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyImageID: privateImage12345Fixture,
			keyDryRun:  true,
		}))
		requireNoError(t, err)
		assertTrue(t, result.IsError)
		assertErrorContains(t, result, "regions is required")
	})
}

func TestLinodeImageUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageUpdateTool(&config.Config{})
		assertContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/images/"+privateImage12345Fixture,
			linode.Image{ID: privateImage12345Fixture})
		_, _, handler := tools.NewLinodeImageUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyImageID: privateImage12345Fixture,
			keyLabel:   testRenamedLabel,
			keyDryRun:  true,
		}))
		requireNoError(t, err)
		requireFalse(t, result.IsError)

		var body map[string]any
		requireNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assertEqual(t, "linode_image_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assertEqual(t, "PUT", would["method"])
		assertEqual(t, "/images/"+privateImage12345Fixture, would["path"])
		assertEqual(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates editable field", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyImageID: privateImage12345Fixture,
			keyDryRun:  true,
		}))
		requireNoError(t, err)
		assertTrue(t, result.IsError)
		assertErrorContains(t, result, "at least one of label, description, or tags is required")
	})
}

func TestLinodeImageShareGroupCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageShareGroupCreateTool(&config.Config{})
		assertContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageShareGroupCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  "my-sharegroup",
			keyDryRun: true,
		}))
		requireNoError(t, err)
		requireFalse(t, result.IsError)

		var body map[string]any
		requireNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assertEqual(t, "linode_image_sharegroup_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assertEqual(t, "POST", would["method"])
		assertEqual(t, "/images/sharegroups", would["path"])
		assertNil(t, body["current_state"], "create has no existing resource to preview")
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageShareGroupCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		requireNoError(t, err)
		assertTrue(t, result.IsError)
		assertErrorContains(t, result, "label is required")
	})
}

func TestLinodeImageShareGroupImagesAddToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageShareGroupImagesAddTool(&config.Config{})
		assertContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without adding", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, imageSGGetPath, linode.ImageShareGroup{ID: 123})
		_, _, handler := tools.NewLinodeImageShareGroupImagesAddTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: float64(123),
			keyImages:       `[{"id":"private/12345"}]`,
			keyDryRun:       true,
		}))
		requireNoError(t, err)
		requireFalse(t, result.IsError)

		var body map[string]any
		requireNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assertEqual(t, "linode_image_sharegroup_images_add", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assertEqual(t, "POST", would["method"])
		assertEqual(t, imageSGGetPath+"/images", would["path"])
		assertEqual(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates sharegroup_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageShareGroupImagesAddTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyImages: `[{"id":"private/12345"}]`,
			keyDryRun: true,
		}))
		requireNoError(t, err)
		assertTrue(t, result.IsError)
	})
}

func TestLinodeImageShareGroupImageUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageShareGroupImageUpdateTool(&config.Config{})
		assertContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, imageSGGetPath, linode.ImageShareGroup{ID: 123})
		_, _, handler := tools.NewLinodeImageShareGroupImageUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: float64(123),
			keyImageID:      imageShareGroupImageIDFixture,
			keyLabel:        testRenamedLabel,
			keyDryRun:       true,
		}))
		requireNoError(t, err)
		requireFalse(t, result.IsError)

		var body map[string]any
		requireNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assertEqual(t, "linode_image_sharegroup_image_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assertEqual(t, "PUT", would["method"])
		assertEqual(t, imageSGGetPath+"/images/"+imageShareGroupImageIDFixture, would["path"])
		assertEqual(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates shared image_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageShareGroupImageUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: float64(123),
			keyImageID:      "private/12345",
			keyLabel:        testRenamedLabel,
			keyDryRun:       true,
		}))
		requireNoError(t, err)
		assertTrue(t, result.IsError)
		assertErrorContains(t, result, "image_id must match shared/")
	})
}

func TestLinodeImageShareGroupMembersAddToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageShareGroupMembersAddTool(&config.Config{})
		assertContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without adding, fetches parent not token", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, imageSGGetPath, linode.ImageShareGroup{ID: 123})
		_, _, handler := tools.NewLinodeImageShareGroupMembersAddTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: float64(123),
			keyLabel:        "member-1",
			keyToken:        "member-token",
			keyDryRun:       true,
		}))
		requireNoError(t, err)
		requireFalse(t, result.IsError)

		text := dryRunResultText(t, result)

		var body map[string]any
		requireNoError(t, json.Unmarshal([]byte(text), &body))
		assertEqual(t, "linode_image_sharegroup_members_add", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assertEqual(t, "POST", would["method"])
		assertEqual(t, imageSGGetPath+"/members", would["path"])
		assertEqual(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
		assertNotContains(t, text, "member-token", "dry_run preview must not echo the membership token")
	})

	t.Run("still validates token", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageShareGroupMembersAddTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: float64(123),
			keyLabel:        "member-1",
			keyDryRun:       true,
		}))
		requireNoError(t, err)
		assertTrue(t, result.IsError)
		assertErrorContains(t, result, "token is required")
	})
}

func TestLinodeImageShareGroupUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageShareGroupUpdateTool(&config.Config{})
		assertContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, imageSGGetPath, linode.ImageShareGroup{ID: 123})
		_, _, handler := tools.NewLinodeImageShareGroupUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: float64(123),
			keyLabel:        testRenamedLabel,
			keyDryRun:       true,
		}))
		requireNoError(t, err)
		requireFalse(t, result.IsError)

		var body map[string]any
		requireNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assertEqual(t, "linode_image_sharegroup_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assertEqual(t, "PUT", would["method"])
		assertEqual(t, imageSGGetPath, would["path"])
		assertEqual(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates editable field", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageShareGroupUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: float64(123),
			keyDryRun:       true,
		}))
		requireNoError(t, err)
		assertTrue(t, result.IsError)
		assertErrorContains(t, result, "at least one of label or description is required")
	})
}

func TestLinodeImageShareGroupMemberUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageShareGroupMemberUpdateTool(&config.Config{})
		assertContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without updating, fetches parent not token", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, imageSGGetPath, linode.ImageShareGroup{ID: 123})
		_, _, handler := tools.NewLinodeImageShareGroupMemberUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: float64(123),
			keyTokenUUID:    shareGroupTokenGetUUID,
			keyLabel:        testRenamedLabel,
			keyDryRun:       true,
		}))
		requireNoError(t, err)
		requireFalse(t, result.IsError)

		var body map[string]any
		requireNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assertEqual(t, "linode_image_sharegroup_member_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assertEqual(t, "PUT", would["method"])
		assertEqual(t, imageSGGetPath+"/members/"+shareGroupTokenGetUUID, would["path"])
		assertEqual(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates token_uuid", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageShareGroupMemberUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyShareGroupID: float64(123),
			keyLabel:        testRenamedLabel,
			keyDryRun:       true,
		}))
		requireNoError(t, err)
		assertTrue(t, result.IsError)
	})
}

func TestLinodeImageShareGroupTokenCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageShareGroupTokenCreateTool(&config.Config{})
		assertContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageShareGroupTokenCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyValidForShareGroupUUID: "sg-uuid-1",
			keyDryRun:                 true,
		}))
		requireNoError(t, err)
		requireFalse(t, result.IsError)

		var body map[string]any
		requireNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assertEqual(t, "linode_image_sharegroup_token_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assertEqual(t, "POST", would["method"])
		assertEqual(t, "/images/sharegroups/tokens", would["path"])
		assertNil(t, body["current_state"], "create has no existing resource to preview")
	})

	t.Run("still validates valid_for_sharegroup_uuid", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageShareGroupTokenCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		requireNoError(t, err)
		assertTrue(t, result.IsError)
		assertErrorContains(t, result, "valid_for_sharegroup_uuid is required")
	})
}

func TestLinodeImageShareGroupTokenUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeImageShareGroupTokenUpdateTool(&config.Config{})
		assertContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without updating, fetches parent not token", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/images/sharegroups/tokens/"+shareGroupTokenGetUUID+"/sharegroup",
			linode.ImageShareGroup{ID: 123})
		_, _, handler := tools.NewLinodeImageShareGroupTokenUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyTokenUUID: shareGroupTokenGetUUID,
			keyLabel:     testRenamedLabel,
			keyDryRun:    true,
		}))
		requireNoError(t, err)
		requireFalse(t, result.IsError)

		var body map[string]any
		requireNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assertEqual(t, "linode_image_sharegroup_token_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assertEqual(t, "PUT", would["method"])
		assertEqual(t, "/images/sharegroups/tokens/"+shareGroupTokenGetUUID, would["path"])
		assertEqual(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeImageShareGroupTokenUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyTokenUUID: shareGroupTokenGetUUID,
			keyDryRun:    true,
		}))
		requireNoError(t, err)
		assertTrue(t, result.IsError)
		assertErrorContains(t, result, "label is required")
	})
}
