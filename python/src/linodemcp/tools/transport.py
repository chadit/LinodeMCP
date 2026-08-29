"""The declared transports' engine.

The emitter renders each tool's execute_transport as one spec literal and this
interpreter runs it, so both languages move one file from one declaration.
Mirrors Go's internal/tools/transport.go: same guards, same order, same
measurements reported back.

What the declaration cannot carry stays here and in objectdata: the framing of a
multipart form, the streaming of a presigned transfer, and the sentences a
refused transfer answers with.
"""

from __future__ import annotations

import base64
import binascii
from dataclasses import dataclass, field
from typing import TYPE_CHECKING, Any, cast

from linodemcp import objectdata
from linodemcp.config import ObjectStorageConfig
from linodemcp.linode.routes import contract_for

if TYPE_CHECKING:
    from collections.abc import Mapping

    from linodemcp.config import Config
    from linodemcp.linode import RetryableClient

# The single-part ceiling and the presign lifetime a config left at zero. Go
# reads the same two numbers off internal/config.
_DEFAULT_MAX_SINGLE_PART_BYTES = 5 * 1024 * 1024 * 1024
_DEFAULT_PRESIGN_TTL_SECONDS = 3600


@dataclass(frozen=True)
class MultipartUpload:
    """A form upload framed from a local file's CONTENTS."""

    file_argument: str
    part_name: str


@dataclass(frozen=True)
class RawBody:
    """The resource itself as the body, under one content type.

    The argument and the member named here hold standard base64, because that is
    the only way a JSON tool schema can advertise bytes.
    """

    content_type: str
    up: bool = False
    source_argument: str = ""
    answer_field: str = ""


@dataclass(frozen=True)
class PresignTransfer:
    """A minted-URL transfer: the declared POST signs it, the transfer follows."""

    url_field: str
    local_path_argument: str
    up: bool = False
    content_type_argument: str = ""
    overwrite_argument: str = ""
    size_field: str = ""
    etag_field: str = ""
    constants: Mapping[str, str] = field(default_factory=dict[str, str])


TransportSpec = MultipartUpload | RawBody | PresignTransfer


@dataclass(frozen=True)
class PresignPreview:
    """What a dry run measured off the local end of a presigned upload.

    Exactly one member is filled: the size the transfer would send, or the
    sentence the guard refuses it with. That is what lets a declared wording
    read the size and drop itself when the guard spoke instead.
    """

    size_bytes: str = ""
    refusal: str = ""


def preview_presign_source(
    cfg: Config,
    arguments: dict[str, Any],
    body: dict[str, Any] | None,
    local_path_argument: str,
) -> PresignPreview:
    """Run the upload guard the live transfer runs, before any call.

    The presign body is filled the way the live call fills it, so the preview
    describes the request the tool would make rather than the one the caller
    spelled. Nothing is opened: a preview that cannot say "this file will be
    refused" has told the caller nothing they needed, and one that streams the
    file has made half the call.
    """
    settings = object_transfer_settings(cfg.object_storage)
    fill_presign_body(body, settings)

    try:
        source = objectdata.inspect(
            str(arguments.get(local_path_argument, "")), settings.filesystem_root
        )
        objectdata.check_single_part(source.size_bytes, settings.max_single_part_bytes)
    except objectdata.ObjectDataError as err:
        return PresignPreview(refusal=str(err))

    return PresignPreview(size_bytes=str(source.size_bytes))


async def run_transport(
    spec: TransportSpec,
    client: RetryableClient,
    tool: str,
    arguments: dict[str, Any],
    values: tuple[object, ...],
    body: dict[str, Any] | None,
) -> Mapping[str, Any]:
    """Make the tool's live call the way its declared transport travels.

    Answers the response members the transfer filled, which is nothing at all
    for an arm the API reports by status.
    """
    retry = not contract_for(tool).retry_disabled

    if isinstance(spec, MultipartUpload):
        await client.route_multipart(
            tool,
            *values,
            part_name=spec.part_name,
            file_path=str(arguments.get(spec.file_argument, "")),
            retry=retry,
        )
        return {}

    if isinstance(spec, RawBody):
        return await _run_raw_body(spec, client, tool, arguments, values, retry=retry)

    return await _run_presign(spec, client, tool, arguments, values, body, retry=retry)


async def _run_raw_body(
    spec: RawBody,
    client: RetryableClient,
    tool: str,
    arguments: dict[str, Any],
    values: tuple[object, ...],
    *,
    retry: bool,
) -> Mapping[str, Any]:
    """Send the decoded argument, or read the answer and encode it."""
    if spec.up:
        await client.route_raw_body(
            tool,
            *values,
            content_type=spec.content_type,
            payload=_decoded(arguments.get(spec.source_argument)),
            retry=retry,
        )
        return {}

    raw = await client.route_raw_body_read(
        tool, *values, accept=spec.content_type, retry=retry
    )

    return {spec.answer_field: base64.b64encode(raw).decode("ascii")}


def _decoded(value: object) -> bytes:
    """Decode base64 text into the bytes a raw-body transfer sends.

    Text the decoder refuses answers no bytes, because the message rules have
    already accepted this value by here and a second sentence from the transport
    would be one the caller never reads.
    """
    if not isinstance(value, str):
        return b""

    try:
        # validate=True matches Go's strict decoder rather than silently
        # dropping characters outside the alphabet.
        return base64.b64decode(value, validate=True)
    except (binascii.Error, ValueError):
        return b""


async def _run_presign(
    spec: PresignTransfer,
    client: RetryableClient,
    tool: str,
    arguments: dict[str, Any],
    values: tuple[object, ...],
    body: dict[str, Any] | None,
    *,
    retry: bool,
) -> Mapping[str, Any]:
    """Ask the API for a presigned URL, then move the local file through it.

    The local end is resolved before the presign call, so an oversized source or
    an occupied destination costs no request at all.
    """
    settings = object_transfer_settings(client.object_storage)
    local_path = str(arguments.get(spec.local_path_argument, ""))

    if spec.up:
        source = objectdata.inspect(local_path, settings.filesystem_root)
        objectdata.check_single_part(source.size_bytes, settings.max_single_part_bytes)
        moved = await objectdata.upload(
            await _minted_url(spec, client, tool, values, body, settings, retry=retry),
            source,
            str(
                arguments.get(spec.content_type_argument)
                or objectdata.DEFAULT_CONTENT_TYPE
            ),
            settings.transfer_timeout,
        )
        return _measured(spec, moved.size_bytes, moved.etag)

    destination = objectdata.resolve_destination(
        local_path,
        settings.filesystem_root,
        overwrite=bool(arguments.get(spec.overwrite_argument, False)),
    )
    fetched = await objectdata.download(
        await _minted_url(spec, client, tool, values, body, settings, retry=retry),
        destination,
        settings.transfer_timeout,
    )

    return _measured(spec, fetched.size_bytes, fetched.etag)


def _measured(spec: PresignTransfer, size_bytes: int, etag: str) -> Mapping[str, Any]:
    """What a transfer reports back, under the members the tool declared."""
    return {spec.size_field: size_bytes, spec.etag_field: etag, **spec.constants}


async def _minted_url(
    spec: PresignTransfer,
    client: RetryableClient,
    tool: str,
    values: tuple[object, ...],
    body: dict[str, Any] | None,
    settings: ObjectStorageConfig,
    *,
    retry: bool,
) -> str:
    """Ask the API for the presigned URL, refusing an answer that is not an object.

    route_raw hands back whatever decoded, so an array or a scalar would reach
    .get() and answer an AttributeError naming a Python type. Go's decode of the
    same body fails with this sentence, so the refusal is worded to match.
    """
    presigned = await client.route_raw(
        tool, *values, body=fill_presign_body(body, settings), retry=retry
    )

    if not isinstance(presigned, dict):
        subject = tool.removeprefix("linode_").replace("_", " ")
        msg = (
            f"failed to unmarshal {subject} object: response body is not a JSON object"
        )
        raise TypeError(msg)

    return str(cast("dict[str, Any]", presigned).get(spec.url_field, ""))


def object_transfer_settings(settings: ObjectStorageConfig) -> ObjectStorageConfig:
    """Fill any data-plane budget the config left at zero.

    Both the transfer and the preview describing it resolve through here,
    because a preview naming one presign lifetime while the transfer requested
    another would report a call the tool does not make.
    """
    return ObjectStorageConfig(
        filesystem_root=settings.filesystem_root,
        max_single_part_bytes=settings.max_single_part_bytes
        or _DEFAULT_MAX_SINGLE_PART_BYTES,
        transfer_timeout=objectdata.resolved_timeout(settings.transfer_timeout),
        presign_ttl_seconds=settings.presign_ttl_seconds
        or _DEFAULT_PRESIGN_TTL_SECONDS,
    )


def fill_presign_body(
    body: dict[str, Any] | None, settings: ObjectStorageConfig
) -> dict[str, Any] | None:
    """Add the members the caller may omit but the presign request has to carry.

    Content-Type especially: the signature covers it, so a URL signed without one
    and then used with one is refused by the endpoint.
    """
    if body is None:
        return None

    body.setdefault("content_type", objectdata.DEFAULT_CONTENT_TYPE)
    body.setdefault("expires_in", settings.presign_ttl_seconds)

    return body
