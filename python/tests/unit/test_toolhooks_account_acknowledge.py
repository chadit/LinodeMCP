"""The base64 decode and the raw-PNG upload the OAuth thumbnail hooks own.

Every rule these tools refuse an argument by is declared on the input message
and evaluated by ``linodemcp.tools.constraints``, apart from the thumbnail
image: decoding base64 is not something a rule can express. The Go twin is
``account_acknowledge_test.go`` in ``go/internal/toolhooks``.
"""

from __future__ import annotations

from typing import Any
from unittest.mock import AsyncMock

from linodemcp import toolhooks

OAUTH_CLIENT_ID = "abc123"
# HELLO_BASE64 decodes to b"hello", which is what the wire test looks for.
HELLO_BASE64 = "aGVsbG8="


def _mocked_client(**returns: Any) -> Any:
    """Build the patched RetryableClient a fetching preview reads through."""
    client = AsyncMock()
    for name, value in returns.items():
        getattr(client, name).return_value = value
    client.__aenter__.return_value = client
    client.__aexit__.return_value = None
    return client


async def test_thumbnail_update_execute_sends_the_decoded_image() -> None:
    """The route takes the raw PNG, so the bytes travel rather than the text.

    The Go twin is TestLinodeAccountOauthClientThumbnailUpdateExecuteSendsTheRawImage.
    """
    client = _mocked_client(update_account_oauth_client_thumbnail={})

    await toolhooks.linode_account_oauth_client_thumbnail_update_execute(
        client,
        {"client_id": OAUTH_CLIENT_ID, "thumbnail_png_base64": HELLO_BASE64},
        (OAUTH_CLIENT_ID,),
        {"thumbnail_png_base64": HELLO_BASE64},
    )

    client.update_account_oauth_client_thumbnail.assert_awaited_once_with(
        OAUTH_CLIENT_ID, b"hello"
    )


async def test_thumbnail_get_execute_answers_with_the_encoded_image() -> None:
    """The route answers with bytes, so the hook hands back the text they encode.

    The Go twin is
    TestLinodeAccountOauthClientThumbnailGetExecuteEncodesTheRawImage.
    """
    client = _mocked_client(
        get_account_oauth_client_thumbnail={"thumbnail_png_base64": HELLO_BASE64}
    )

    assembled = await toolhooks.linode_account_oauth_client_thumbnail_get_execute(
        client, (OAUTH_CLIENT_ID,)
    )

    # Exactly the declared member: the driver refuses anything else, and
    # client_id is placed there from the call rather than here.
    assert assembled == {"thumbnail_png_base64": HELLO_BASE64}
    client.get_account_oauth_client_thumbnail.assert_awaited_once_with(OAUTH_CLIENT_ID)
