"""The upload the support ticket attachment hook owns.

Its route takes multipart/form-data framed from the file's contents rather than
the JSON body the emitter derives, which is why the step is still written out.
The Go twin is ``support_test.go`` in ``go/internal/toolhooks``.
"""

from __future__ import annotations

from unittest.mock import AsyncMock

from linodemcp import toolhooks

TICKET_ID = 123
# A path the upload never opens: every case here stops at the client boundary.
ATTACHMENT_PATH = "/var/log/linodemcp/report.txt"


async def test_attachment_execute_uploads_through_the_plain_client() -> None:
    """The tool declares retry_disabled, so the upload goes through the client
    the retrying wrapper wraps rather than the wrapper itself.

    The path comes off the call because the JSON body is not what travels: the
    route takes multipart/form-data built from the file's contents.

    The Go twin is TestSupportTicketAttachmentCreateExecuteUploadsTheFileContents.
    """
    client = AsyncMock()

    await toolhooks.linode_support_ticket_attachment_create_execute(
        client,
        {"ticket_id": TICKET_ID, "file": ATTACHMENT_PATH},
        (TICKET_ID,),
        {"file": ATTACHMENT_PATH},
    )

    client.client.create_support_ticket_attachment.assert_awaited_once_with(
        TICKET_ID, ATTACHMENT_PATH
    )
    client.create_support_ticket_attachment.assert_not_awaited()
