"""The declared transports, run against a mocked client.

Each arm is checked at the client boundary: what the engine hands the routed
primitive, and what it answers with. The Go twin is ``transport_test.go`` in
``go/internal/tools``.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any
from unittest.mock import AsyncMock

import pytest

from linodemcp.config import Config, ObjectStorageConfig
from linodemcp.tools.transport import (
    MultipartUpload,
    PresignRemove,
    PresignTransfer,
    RawBody,
    preview_presign_source,
    run_transport,
)

if TYPE_CHECKING:
    from pathlib import Path

TICKET_ID = 123
# A path the upload never opens: every case here stops at the client boundary.
ATTACHMENT_PATH = "/var/log/linodemcp/report.txt"
OAUTH_CLIENT_ID = "abc123"
# HELLO_BASE64 decodes to b"hello", which is what the wire cases look for.
HELLO_BASE64 = "aGVsbG8="

ATTACHMENT_TOOL = "linode_support_ticket_attachment_create"
THUMBNAIL_UPDATE_TOOL = "linode_account_oauth_client_thumbnail_update"
THUMBNAIL_GET_TOOL = "linode_account_oauth_client_thumbnail_get"
DOWNLOAD_TOOL = "linode_object_storage_object_download"
DELETE_TOOL = "linode_object_storage_object_delete"


def _mocked_client(**returns: Any) -> Any:
    """Build the patched RetryableClient a transport reaches through."""
    client = AsyncMock()
    for name, value in returns.items():
        getattr(client, name).return_value = value
    return client


async def test_multipart_sends_the_named_file_under_the_declared_part() -> None:
    """The path comes off the arguments because the JSON body is not what
    travels: the route takes multipart/form-data built from the file's contents.

    The tool declares retry_disabled, so the engine selects one protected
    attempt rather than the retrying default.
    """
    client = _mocked_client()

    filled = await run_transport(
        MultipartUpload(file_argument="file", part_name="file"),
        client,
        ATTACHMENT_TOOL,
        {"ticket_id": TICKET_ID, "file": ATTACHMENT_PATH},
        (TICKET_ID,),
        {"file": ATTACHMENT_PATH},
    )

    assert filled == {}
    client.route_multipart.assert_awaited_once_with(
        ATTACHMENT_TOOL,
        TICKET_ID,
        part_name="file",
        file_path=ATTACHMENT_PATH,
        retry=False,
    )


async def test_raw_body_up_sends_the_decoded_image() -> None:
    """The route takes the raw PNG, so the bytes travel rather than the text."""
    client = _mocked_client()

    filled = await run_transport(
        RawBody(
            content_type="image/png",
            up=True,
            source_argument="thumbnail_png_base64",
        ),
        client,
        THUMBNAIL_UPDATE_TOOL,
        {"client_id": OAUTH_CLIENT_ID, "thumbnail_png_base64": HELLO_BASE64},
        (OAUTH_CLIENT_ID,),
        {"thumbnail_png_base64": HELLO_BASE64},
    )

    assert filled == {}
    client.route_raw_body.assert_awaited_once_with(
        THUMBNAIL_UPDATE_TOOL,
        OAUTH_CLIENT_ID,
        content_type="image/png",
        payload=b"hello",
        retry=False,
    )


@pytest.mark.parametrize(
    "value",
    [
        pytest.param("not base64!", id="unusable text"),
        pytest.param(None, id="absent argument"),
    ],
)
async def test_raw_body_up_sends_no_bytes_for_text_the_rules_would_refuse(
    value: str | None,
) -> None:
    """The message rules accepted this value before the transport saw it, so a
    second sentence here would be one the caller never reads."""
    client = _mocked_client()
    arguments: dict[str, Any] = {"client_id": OAUTH_CLIENT_ID}
    if value is not None:
        arguments["thumbnail_png_base64"] = value

    await run_transport(
        RawBody(
            content_type="image/png",
            up=True,
            source_argument="thumbnail_png_base64",
        ),
        client,
        THUMBNAIL_UPDATE_TOOL,
        arguments,
        (OAUTH_CLIENT_ID,),
        None,
    )

    assert client.route_raw_body.await_args.kwargs["payload"] == b""


async def test_raw_body_down_answers_with_the_encoded_image() -> None:
    """The route answers with bytes, so the transfer hands back the text they
    encode, under exactly the member the contract declares."""
    client = _mocked_client(route_raw_body_read=b"hello")

    filled = await run_transport(
        RawBody(content_type="image/png", answer_field="thumbnail_png_base64"),
        client,
        THUMBNAIL_GET_TOOL,
        {"client_id": OAUTH_CLIENT_ID},
        (OAUTH_CLIENT_ID,),
        None,
    )

    assert filled == {"thumbnail_png_base64": HELLO_BASE64}
    client.route_raw_body_read.assert_awaited_once_with(
        THUMBNAIL_GET_TOOL, OAUTH_CLIENT_ID, accept="image/png", retry=True
    )


async def test_presign_refuses_an_answer_that_is_not_an_object() -> None:
    """route_raw hands back whatever decoded, so an array would reach .get() and
    answer an AttributeError naming a Python type. Go's decode of the same body
    fails with this sentence, so the refusal is worded to match."""
    client = _mocked_client(route_raw=[])
    client.object_storage.filesystem_root = ""
    client.object_storage.max_single_part_bytes = 0
    client.object_storage.transfer_timeout = 0
    client.object_storage.presign_ttl_seconds = 0

    with pytest.raises(TypeError, match="object storage object download"):
        await run_transport(
            PresignTransfer(
                url_field="url",
                local_path_argument="dest_path",
                overwrite_argument="overwrite",
                size_field="size_bytes",
                etag_field="etag",
            ),
            client,
            DOWNLOAD_TOOL,
            {"dest_path": "downloads/landed.bin"},
            ("us-east", "artifacts"),
            {},
        )


async def test_presign_remove_presigns_then_deletes(monkeypatch: Any) -> None:
    """The two legs and their order: one Linode call, then the DELETE to the URL
    that call answered with. The engine fills the two body members the caller may
    omit, so the request the endpoint signs is the one it accepts.

    Go's twin is TestRunPresignRemovePresignsThenDeletes.
    """
    client = _mocked_client(route_raw={"url": "https://example.test/key?sig=fixture"})
    client.object_storage = ObjectStorageConfig()

    removed: list[tuple[str, float]] = []

    async def fake_remove(url: str, timeout: float) -> None:
        removed.append((url, timeout))

    monkeypatch.setattr("linodemcp.objectdata.remove", fake_remove)

    body: dict[str, Any] = {"name": "releases/app.tar.gz", "method": "DELETE"}

    filled = await run_transport(
        PresignRemove(url_field="url"),
        client,
        DELETE_TOOL,
        {"name": "releases/app.tar.gz"},
        ("us-east", "artifacts"),
        body,
    )

    assert filled == {}
    assert removed == [("https://example.test/key?sig=fixture", 1800.0)]
    client.route_raw.assert_awaited_once_with(
        DELETE_TOOL,
        "us-east",
        "artifacts",
        body={
            "name": "releases/app.tar.gz",
            "method": "DELETE",
            "content_type": "application/octet-stream",
            "expires_in": 3600,
        },
        retry=False,
    )


async def test_presign_remove_refuses_an_answer_that_is_not_an_object() -> None:
    """The removal arm mints through the same reader the transfers do, so a bare
    array is refused with the sentence Go's decode of the same body answers."""
    client = _mocked_client(route_raw=[])
    client.object_storage = ObjectStorageConfig()

    with pytest.raises(TypeError, match="object storage object delete"):
        await run_transport(
            PresignRemove(url_field="url"),
            client,
            DELETE_TOOL,
            {"name": "releases/app.tar.gz"},
            ("us-east", "artifacts"),
            {},
        )


# The presigned upload's dry run: the guard the live transfer runs, taken before
# any call. Go's twin is transport_preview_test.go.
UPLOAD_ARGUMENTS = {"name": "releases/app.tar.gz", "label": "artifacts"}


def test_preview_presign_source_measures_the_source_and_fills_the_body(
    tmp_path: Path,
) -> None:
    """The defaults apply with an untouched config, so the preview describes the
    request the live call would sign."""
    source = tmp_path / "small.bin"
    source.write_text("hello object storage")
    body: dict[str, Any] = {"name": "k"}

    transfer = preview_presign_source(
        Config(), {**UPLOAD_ARGUMENTS, "source_path": str(source)}, body, "source_path"
    )

    assert transfer.size_bytes == "20"
    assert transfer.refusal == ""
    assert body == {
        "name": "k",
        "content_type": "application/octet-stream",
        "expires_in": 3600,
    }


def test_preview_presign_source_keeps_what_the_caller_sent(tmp_path: Path) -> None:
    """A default fills a gap; it never overrides what the caller asked for."""
    source = tmp_path / "small.bin"
    source.write_text("hello object storage")
    body: dict[str, Any] = {
        "name": "k",
        "content_type": "text/plain",
        "expires_in": 900,
    }

    preview_presign_source(
        Config(), {**UPLOAD_ARGUMENTS, "source_path": str(source)}, body, "source_path"
    )

    assert body["content_type"] == "text/plain"
    assert body["expires_in"] == 900


def test_preview_presign_source_survives_a_tool_with_no_body(tmp_path: Path) -> None:
    """A missing body has nothing to default into and must not crash."""
    source = tmp_path / "small.bin"
    source.write_text("hello object storage")

    transfer = preview_presign_source(
        Config(), {**UPLOAD_ARGUMENTS, "source_path": str(source)}, None, "source_path"
    )

    assert transfer.size_bytes == "20"


def test_preview_presign_source_refuses_what_the_transfer_would_refuse(
    tmp_path: Path,
) -> None:
    """A source the guard refuses answers the guard's sentence and no size."""
    big = tmp_path / "big.bin"
    big.write_bytes(b"a" * 2048)
    ceiling = Config(object_storage=ObjectStorageConfig(max_single_part_bytes=1024))

    over = preview_presign_source(
        ceiling, {**UPLOAD_ARGUMENTS, "source_path": str(big)}, None, "source_path"
    )
    missing = preview_presign_source(
        Config(),
        {**UPLOAD_ARGUMENTS, "source_path": str(tmp_path / "absent.bin")},
        None,
        "source_path",
    )

    assert over.size_bytes == ""
    assert "single-part upload ceiling" in over.refusal
    assert missing.size_bytes == ""
    assert "no readable file" in missing.refusal
