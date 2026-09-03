"""Unit tests for the Object Storage data-plane transfer helper.

Go's twin lives at ``go/internal/objectdata/objectdata_test.go`` and asserts the
same facts, because the two helpers have to refuse the same inputs with the same
sentences for one behavior fixture to judge both.
"""

from __future__ import annotations

from contextlib import contextmanager
from typing import TYPE_CHECKING, Any

import httpx
import pytest

from linodemcp import objectdata

if TYPE_CHECKING:
    from collections.abc import Generator
    from pathlib import Path


@contextmanager
def _mock_transport(transport: httpx.MockTransport) -> Generator[None]:
    """Route every AsyncClient the helper builds through a fake transport.

    The helper makes its own client on purpose (the presigned URL must not carry
    the Linode token), so there is no client to inject; patching the constructor
    is what reaches it.
    """
    original = httpx.AsyncClient.__init__

    def patched(self: httpx.AsyncClient, **kwargs: Any) -> None:
        original(self, transport=transport, **kwargs)

    httpx.AsyncClient.__init__ = patched  # type: ignore[method-assign]
    try:
        yield
    finally:
        httpx.AsyncClient.__init__ = original  # type: ignore[method-assign]


def _write(tmp_path: Path, name: str, content: str) -> Path:
    """Put one file on disk and answer its path."""
    target = tmp_path / name
    target.write_text(content)
    return target


@pytest.mark.asyncio
async def test_upload_sends_the_file_bytes_to_the_presigned_url(
    tmp_path: Path,
) -> None:
    """The byte-move proof: the transport receives the file's exact bytes.

    A transfer that sent nothing, truncated, or re-encoded the file fails here
    rather than reporting a success it did not achieve.
    """
    content = "hello object storage"
    source = objectdata.inspect(str(_write(tmp_path, "small.bin", content)), "")

    received: dict[str, Any] = {}

    async def handler(request: httpx.Request) -> httpx.Response:
        received["body"] = await request.aread()
        received["method"] = request.method
        received["content_type"] = request.headers.get("content-type")
        return httpx.Response(
            200, headers={"ETag": '"5eb63bbbe01eeed093cb22bb8f5acdc3"'}
        )

    with _mock_transport(httpx.MockTransport(handler)):
        result = await objectdata.upload(
            "http://linode.test/artifacts/small.bin?sig=fixture",
            source,
            "text/plain",
            30.0,
        )

    assert received["body"] == content.encode()
    assert received["method"] == "PUT"
    assert received["content_type"] == "text/plain"
    assert result.size_bytes == len(content)
    assert result.etag == "5eb63bbbe01eeed093cb22bb8f5acdc3"


def test_inspect_reports_the_resolved_path_and_size(tmp_path: Path) -> None:
    """A readable file reports the size a transfer would send."""
    source = objectdata.inspect(str(_write(tmp_path, "small.bin", "12345")), "")
    assert source.size_bytes == 5


def test_inspect_refuses_a_missing_file(tmp_path: Path) -> None:
    """A path with nothing behind it is refused before any call."""
    with pytest.raises(objectdata.SourceMissingError):
        objectdata.inspect(str(tmp_path / "absent.bin"), "")


def test_inspect_refuses_a_directory(tmp_path: Path) -> None:
    """A directory would stream bytes the caller did not mean to store."""
    with pytest.raises(objectdata.SourceNotRegularError):
        objectdata.inspect(str(tmp_path), "")


def test_inspect_accepts_a_file_inside_the_configured_root(tmp_path: Path) -> None:
    """A root that contains the file confines nothing that matters."""
    root = tmp_path / "root"
    root.mkdir()
    source = objectdata.inspect(str(_write(root, "inside.bin", "ok")), str(root))
    assert source.size_bytes == 2


def test_inspect_refuses_a_file_outside_the_configured_root(tmp_path: Path) -> None:
    """A file the root does not contain is refused when a root is set."""
    root = tmp_path / "root"
    root.mkdir()
    outside = _write(tmp_path, "outside.bin", "ok")

    with pytest.raises(objectdata.SourceOutsideRootError):
        objectdata.inspect(str(outside), str(root))


def test_inspect_refuses_a_symlink_pointing_out_of_the_root(tmp_path: Path) -> None:
    """The link sits inside the root, so only resolving it first catches it."""
    root = tmp_path / "root"
    root.mkdir()
    outside = _write(tmp_path, "target.bin", "ok")
    link = root / "link.bin"
    link.symlink_to(outside)

    with pytest.raises(objectdata.SourceOutsideRootError):
        objectdata.inspect(str(link), str(root))


def test_inspect_refuses_a_sibling_sharing_the_root_name_prefix(
    tmp_path: Path,
) -> None:
    """A text prefix is not containment: /srv/data-other is not in /srv/data."""
    root = tmp_path / "data"
    sibling = tmp_path / "data-other"
    root.mkdir()
    sibling.mkdir()

    with pytest.raises(objectdata.SourceOutsideRootError):
        objectdata.inspect(str(_write(sibling, "file.bin", "ok")), str(root))


@pytest.mark.parametrize(
    ("size", "ceiling", "refused"),
    [(1023, 1024, False), (1024, 1024, False), (1025, 1024, True)],
    ids=["under", "exactly-at", "over"],
)
def test_check_single_part(size: int, ceiling: int, refused: bool) -> None:
    """The ceiling refuses only what one presigned PUT cannot carry."""
    if not refused:
        objectdata.check_single_part(size, ceiling)
        return

    with pytest.raises(objectdata.AboveSinglePartCeilingError) as caught:
        objectdata.check_single_part(size, ceiling)

    # The refusal has to be actionable: both numbers, and what is missing rather
    # than a tool that does not exist.
    reported = str(caught.value)
    assert "1025" in reported
    assert "1024" in reported
    assert "multipart upload is not implemented yet" in reported


def test_inspect_refuses_when_the_configured_root_does_not_exist(
    tmp_path: Path,
) -> None:
    """An unresolvable root confines everything rather than nothing.

    A misconfigured root is the one case where failing open would silently drop
    the confinement the operator asked for.
    """
    with pytest.raises(objectdata.SourceOutsideRootError):
        objectdata.inspect(
            str(_write(tmp_path, "file.bin", "ok")), str(tmp_path / "absent-root")
        )


@pytest.mark.asyncio
async def test_upload_refuses_a_non_success_status(tmp_path: Path) -> None:
    """The bucket refusing the PUT is reported, not treated as a stored object."""
    source = objectdata.inspect(str(_write(tmp_path, "small.bin", "bytes")), "")

    async def handler(_request: httpx.Request) -> httpx.Response:
        return httpx.Response(403)

    with (
        _mock_transport(httpx.MockTransport(handler)),
        pytest.raises(objectdata.TransferError) as caught,
    ):
        await objectdata.upload("http://linode.test/k?sig=secret", source, "", 30.0)

    reported = str(caught.value)
    assert "403" in reported
    # The presigned URL is a bearer credential and must not reach error text.
    assert "secret" not in reported


@pytest.mark.asyncio
async def test_upload_refuses_a_transport_failure_without_the_url(
    tmp_path: Path,
) -> None:
    """httpx puts the full URL in its own exception text; the helper must not."""
    source = objectdata.inspect(str(_write(tmp_path, "small.bin", "bytes")), "")

    async def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("refused", request=request)

    with (
        _mock_transport(httpx.MockTransport(handler)),
        pytest.raises(objectdata.TransferError) as caught,
    ):
        await objectdata.upload("http://linode.test/k?sig=secret", source, "", 30.0)

    assert "secret" not in str(caught.value)


def test_resolved_timeout_falls_back_when_config_left_it_at_zero() -> None:
    """Go applies the same fallback, so one config produces one budget."""
    assert objectdata.resolved_timeout(0) == 1800.0
    assert objectdata.resolved_timeout(5.0) == 5.0


@pytest.mark.asyncio
async def test_download_writes_the_served_bytes_to_the_destination(
    tmp_path: Path,
) -> None:
    """The byte-move proof for the download half: the file matches what was sent."""
    body = b"hello object storage"

    async def handler(request: httpx.Request) -> httpx.Response:
        assert request.method == "GET"
        return httpx.Response(200, content=body, headers={"ETag": '"abc123"'})

    destination = tmp_path / "landed.bin"

    with _mock_transport(httpx.MockTransport(handler)):
        result = await objectdata.download(
            "http://linode.test/artifacts/key?sig=fixture", destination, 30.0
        )

    assert destination.read_bytes() == body
    assert result.size_bytes == len(body)
    assert result.etag == "abc123"


@pytest.mark.asyncio
async def test_download_leaves_no_file_when_the_bucket_refuses(tmp_path: Path) -> None:
    """A refused download must not leave a truncated file at the caller's path."""

    async def handler(_request: httpx.Request) -> httpx.Response:
        return httpx.Response(403)

    destination = tmp_path / "landed.bin"

    with (
        _mock_transport(httpx.MockTransport(handler)),
        pytest.raises(objectdata.TransferError) as caught,
    ):
        await objectdata.download(
            "http://linode.test/artifacts/key?sig=secret", destination, 30.0
        )

    assert "403" in str(caught.value)
    # The presigned URL is a bearer credential and must not reach error text.
    assert "secret" not in str(caught.value)
    assert not destination.exists()


def test_resolve_destination_refuses_an_existing_file_without_overwrite(
    tmp_path: Path,
) -> None:
    """An occupied destination is refused before any call goes out."""
    occupied = _write(tmp_path, "there.bin", "x")

    with pytest.raises(objectdata.DestinationExistsError):
        objectdata.resolve_destination(str(occupied), "", overwrite=False)


def test_resolve_destination_accepts_an_existing_file_with_overwrite(
    tmp_path: Path,
) -> None:
    """Overwrite is what stands in for a confirm gate on a Read-tier tool."""
    occupied = _write(tmp_path, "there.bin", "x")

    assert objectdata.resolve_destination(str(occupied), "", overwrite=True) == occupied


def test_resolve_destination_refuses_a_directory(tmp_path: Path) -> None:
    """A directory cannot be written to, even with overwrite."""
    with pytest.raises(objectdata.DestinationUnwritableError):
        objectdata.resolve_destination(str(tmp_path), "", overwrite=True)


def test_resolve_destination_refuses_a_path_outside_the_root(tmp_path: Path) -> None:
    """A write outside the root is the case a configured root exists to prevent."""
    root = tmp_path / "root"
    root.mkdir()
    outside = tmp_path / "landed.bin"

    with pytest.raises(objectdata.SourceOutsideRootError):
        objectdata.resolve_destination(str(outside), str(root), overwrite=False)


@pytest.mark.asyncio
async def test_download_refuses_a_transport_failure_without_the_url(
    tmp_path: Path,
) -> None:
    """httpx puts the full URL in its own exception text; the helper must not."""

    async def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("refused", request=request)

    destination = tmp_path / "landed.bin"

    with (
        _mock_transport(httpx.MockTransport(handler)),
        pytest.raises(objectdata.TransferError) as caught,
    ):
        await objectdata.download(
            "http://linode.test/artifacts/key?sig=secret", destination, 30.0
        )

    assert "secret" not in str(caught.value)
    assert not destination.exists()


@pytest.mark.asyncio
async def test_remove_sends_the_delete_the_minted_url_authorizes() -> None:
    """The removal proof: the request that goes out is a DELETE carrying no body,
    no Content-Type, and no Linode token.

    Go's twin is TestRemoveSendsTheDeleteTheMintedURLAuthorizes.
    """
    seen: dict[str, Any] = {}

    async def handler(request: httpx.Request) -> httpx.Response:
        seen["method"] = request.method
        seen["auth"] = request.headers.get("authorization")
        seen["type"] = request.headers.get("content-type")
        seen["body"] = request.content
        return httpx.Response(204)

    with _mock_transport(httpx.MockTransport(handler)):
        await objectdata.remove("http://linode.test/artifacts/key?sig=fixture", 30.0)

    assert seen["method"] == "DELETE"
    assert seen["auth"] is None
    assert seen["type"] is None
    assert seen["body"] == b""


@pytest.mark.asyncio
async def test_remove_reports_a_refusal_without_the_url() -> None:
    """The minted URL is a bearer credential and must not reach error text."""

    async def handler(_request: httpx.Request) -> httpx.Response:
        return httpx.Response(403)

    with (
        _mock_transport(httpx.MockTransport(handler)),
        pytest.raises(objectdata.TransferError) as caught,
    ):
        await objectdata.remove("http://linode.test/artifacts/key?sig=secret", 30.0)

    assert "403" in str(caught.value)
    assert "secret" not in str(caught.value)


@pytest.mark.asyncio
async def test_remove_refuses_a_transport_failure_without_the_url() -> None:
    """httpx puts the full URL in its own exception text; the helper must not."""

    async def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("refused", request=request)

    with (
        _mock_transport(httpx.MockTransport(handler)),
        pytest.raises(objectdata.TransferError) as caught,
    ):
        await objectdata.remove("http://linode.test/artifacts/key?sig=secret", 30.0)

    assert "secret" not in str(caught.value)
