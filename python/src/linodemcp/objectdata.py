"""Move object bytes to the presigned URLs the Linode API mints.

This module sits outside ``linodemcp.linode`` deliberately. ``scripts/_routescan.py``
seeds the route scanner on a call named ``request``, so every function reaching
``httpx.AsyncClient.request`` is walked as a route. A PUT to a URL the API handed
back is not a route this client builds, so the transfer below calls ``.put()``
and must never call ``.request(...)``: doing so would report a route the
contract cannot match.

Go's twin is ``go/internal/objectdata``, off its own scanner's surface for the
same reason. The two keep identical defaults and identical refusal sentences,
because the behavior fixtures judge both against one case.
"""

from __future__ import annotations

import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING

import httpx

if TYPE_CHECKING:
    from collections.abc import AsyncIterator

# What an object is stored under when the caller names none. It is filled into
# the presign body rather than onto the PUT, because the signature covers the
# header and a PUT carrying a type the presign request never named is refused.
DEFAULT_CONTENT_TYPE = "application/octet-stream"

# Read size for the streaming upload. Large enough that a multi-gigabyte file
# does not cost a syscall per kilobyte, small enough that peak memory stays flat.
_CHUNK_BYTES = 1024 * 1024


class ObjectDataError(Exception):
    """Base for every transfer failure, so a caller can catch the family."""


class SourceMissingError(ObjectDataError):
    """The source path names no readable file."""


class SourceNotRegularError(ObjectDataError):
    """The source is a directory, device, or socket rather than a file."""


class SourceOutsideRootError(ObjectDataError):
    """The source resolves outside the configured filesystem root."""


class AboveSinglePartCeilingError(ObjectDataError):
    """The file is larger than one presigned PUT carries."""


class TransferError(ObjectDataError):
    """The PUT against the presigned URL failed."""


@dataclass(frozen=True)
class LocalFile:
    """One resolved upload source: the path after symlink resolution, and size."""

    path: Path
    size_bytes: int


@dataclass(frozen=True)
class UploadResult:
    """What the transfer produced, measured off the stream rather than restated.

    ``etag`` arrives verbatim with its quotes stripped and is not checked here.
    For a single PUT the endpoint's ETag is the stored object's MD5, so a caller
    verifies by hashing their own file and comparing, which tests the round trip
    end to end instead of repeating the server's arithmetic back at it.
    """

    etag: str
    size_bytes: int


def inspect(source_path: str, filesystem_root: str) -> LocalFile:
    """Resolve a source path and report what a transfer would send.

    Opens nothing and calls nothing, so the preview and the transfer describe
    the same file. The path is resolved through symlinks before the root
    comparison, so a link inside the root pointing out of it is refused rather
    than followed. An empty root confines nothing, which is the default: a stdio
    server reads the operator's own paths.
    """
    try:
        resolved = Path(source_path).resolve(strict=True)
    except OSError as err:
        msg = f"source_path names no readable file: {source_path}"
        raise SourceMissingError(msg) from err

    _confine(resolved, filesystem_root)

    if not resolved.is_file():
        msg = f"source_path is not a regular file: {source_path}"
        raise SourceNotRegularError(msg)

    return LocalFile(path=resolved, size_bytes=resolved.stat().st_size)


def _confine(resolved: Path, root: str) -> None:
    """Refuse a resolved path outside root. An empty root confines nothing."""
    if not root:
        return

    try:
        resolved_root = Path(root).resolve(strict=True)
    except OSError as err:
        msg = f"source_path resolves outside the configured filesystem root: {resolved}"
        raise SourceOutsideRootError(msg) from err

    # relative_to is the comparison a string prefix gets wrong: /srv/data-other
    # carries /srv/data as a text prefix and is not inside it.
    if not resolved.is_relative_to(resolved_root):
        msg = f"source_path resolves outside the configured filesystem root: {resolved}"
        raise SourceOutsideRootError(msg)


def check_single_part(size_bytes: int, max_bytes: int) -> None:
    """Refuse a file larger than one presigned PUT carries.

    The sentence names the ceiling and the size the caller has, and says what is
    missing rather than naming a tool that does not exist: an object this large
    needs multipart, and Tier A does not implement it.
    """
    if size_bytes <= max_bytes:
        return

    msg = (
        f"source_path is above the single-part upload ceiling: {size_bytes} bytes"
        f" exceeds the {max_bytes} byte ceiling, and multipart upload is not"
        " implemented yet"
    )
    raise AboveSinglePartCeilingError(msg)


async def upload(
    url: str,
    source: LocalFile,
    content_type: str,
    timeout: float,
) -> UploadResult:
    """Stream the source file to the presigned URL and report what landed.

    A fresh client rather than the API client's: the presigned URL carries its
    authorization in the query string, so attaching the Linode token would put
    an account credential on a host that never needed one.

    No error below reports the URL. It is a bearer credential for its lifetime,
    and httpx puts the full URL in its own exception text, which is why the
    failure is reported by type rather than by re-raising what httpx said.
    """
    sent = _ByteCounter()

    try:
        async with httpx.AsyncClient(timeout=timeout) as client:
            response = await client.put(
                url,
                content=_stream(source.path, sent),
                headers={
                    "Content-Type": content_type,
                    # Set explicitly because the body is an iterator with no
                    # length of its own, and a presigned PUT is signed for a
                    # fixed length rather than a chunked one.
                    "Content-Length": str(source.size_bytes),
                },
            )
    except httpx.HTTPError as err:
        msg = "presigned upload failed: the transfer could not be completed"
        raise TransferError(msg) from err

    if not _accepted(response.status_code):
        msg = (
            f"presigned upload failed: the bucket answered HTTP {response.status_code}"
        )
        raise TransferError(msg)

    return UploadResult(
        etag=response.headers.get("etag", "").strip('"'),
        size_bytes=sent.count,
    )


def _refuse_unaccepted(status_code: int) -> None:
    """Refuse a download whose bucket did not answer with a success status."""
    if _accepted(status_code):
        return

    msg = f"presigned download failed: the bucket answered HTTP {status_code}"
    raise TransferError(msg)


def _accepted(status_code: int) -> bool:
    """Report whether the bucket stored the object.

    A presigned PUT answers 200, and some endpoints answer another 2xx, so the
    whole success class counts rather than one exact code.
    """
    return httpx.codes.OK <= status_code < httpx.codes.MULTIPLE_CHOICES


class _ByteCounter:
    """Counts what the stream actually yielded, which is what the result reports."""

    def __init__(self) -> None:
        self.count = 0


async def _stream(path: Path, sent: _ByteCounter) -> AsyncIterator[bytes]:
    """Yield the file in chunks so peak memory stays flat on a large object."""
    with path.open("rb") as handle:
        while chunk := handle.read(_CHUNK_BYTES):
            sent.count += len(chunk)
            yield chunk


def resolved_timeout(seconds: float) -> float:
    """Fall back to the shared 30 minute budget when config left the value at zero.

    Go applies the same fallback in its hook, so a config that names no transfer
    timeout produces one budget rather than two.
    """
    return seconds if seconds > 0 else 1800.0


__all__ = [
    "DEFAULT_CONTENT_TYPE",
    "AboveSinglePartCeilingError",
    "DestinationExistsError",
    "DestinationUnwritableError",
    "DownloadResult",
    "LocalFile",
    "ObjectDataError",
    "SourceMissingError",
    "SourceNotRegularError",
    "SourceOutsideRootError",
    "TransferError",
    "UploadResult",
    "check_single_part",
    "download",
    "inspect",
    "remove",
    "resolve_destination",
    "resolved_timeout",
    "upload",
]


class DestinationExistsError(ObjectDataError):
    """A file is already at the destination and overwrite was not asked for."""


class DestinationUnwritableError(ObjectDataError):
    """The destination cannot be created or written."""


@dataclass(frozen=True)
class DownloadResult:
    """What the download produced, counted off the stream that was written."""

    etag: str
    size_bytes: int


def resolve_destination(
    dest_path: str, filesystem_root: str, *, overwrite: bool
) -> Path:
    """Refuse the two cases the caller has to fix, before any call goes out.

    A file already at the destination without overwrite, and a destination
    outside the configured root. The parent directory is what gets resolved,
    because the file itself does not exist yet.
    """
    target = Path(dest_path)

    if target.exists():
        if not overwrite:
            msg = f"dest_path already exists and overwrite is false: {dest_path}"
            raise DestinationExistsError(msg)
        if target.is_dir():
            msg = f"dest_path cannot be written: {dest_path} is a directory"
            raise DestinationUnwritableError(msg)

    _confine(target.parent.resolve(strict=False), filesystem_root)

    return target


async def download(
    url: str,
    destination: Path,
    timeout: float,
) -> DownloadResult:
    """Stream the presigned URL into destination.

    Written to a temporary file beside the destination and renamed into place
    only after the whole body has landed, so a transfer that fails midway leaves
    no truncated file at the path the caller named.

    Uses ``.stream()`` rather than ``.get()`` so the body never has to be held
    in memory whole; see this module's docstring for why neither may call
    ``.request()``.
    """
    written = 0
    handle = tempfile.NamedTemporaryFile(  # noqa: SIM115
        dir=destination.parent, prefix=".download-", delete=False
    )
    temporary = Path(handle.name)

    try:
        async with (
            httpx.AsyncClient(timeout=timeout) as client,
            client.stream("GET", url) as response,
        ):
            _refuse_unaccepted(response.status_code)
            async for chunk in response.aiter_bytes(_CHUNK_BYTES):
                written += handle.write(chunk)
            etag = response.headers.get("etag", "").strip('"')
    except httpx.HTTPError as err:
        handle.close()
        temporary.unlink(missing_ok=True)
        msg = "presigned download failed: the transfer could not be completed"
        raise TransferError(msg) from err
    except BaseException:
        handle.close()
        temporary.unlink(missing_ok=True)
        raise
    else:
        handle.close()

    temporary.replace(destination)

    return DownloadResult(etag=etag, size_bytes=written)


async def remove(url: str, timeout: float) -> None:
    """Send the DELETE the minted URL authorizes.

    Nothing is measured because nothing moves: the object is either gone
    afterwards or the bucket said why it is not. The request carries no body and
    no Content-Type, the same shape the download's GET sends.

    A fresh client rather than the API client's, for the reason upload gives:
    the presigned URL carries its authorization in the query string. No error
    below reports the URL, since httpx puts the full URL in its own exception
    text and that URL is a bearer credential.
    """
    try:
        async with httpx.AsyncClient(timeout=timeout) as client:
            response = await client.delete(url)
    except httpx.HTTPError as err:
        msg = "presigned removal failed: the request could not be completed"
        raise TransferError(msg) from err

    if not _accepted(response.status_code):
        msg = (
            f"presigned removal failed: the bucket answered HTTP {response.status_code}"
        )
        raise TransferError(msg)
