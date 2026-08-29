"""The guards a removal's shared driver answers, which no rule can carry.

A path id reaches the driver already parsed, so its sentence is about the value
rather than about the text a rule would see. The Go twin is the destroyID guard
in ``go/internal/tools/destroy.go``, and the sentence below is one both
languages answer.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

from linodemcp.gentools import handle_linode_volume_delete

if TYPE_CHECKING:
    from linodemcp.config import Config


async def test_destroy_driver_refuses_a_negative_id(sample_config: Config) -> None:
    """A below-zero path id is refused rather than spliced into the route.

    The stackscript-delete behavior fixture pins the same sentence for both
    languages.
    """
    result = await handle_linode_volume_delete(
        {"volume_id": -1, "confirm": True, "confirm_bypass_dry_run": True},
        sample_config,
    )

    assert "volume_id must be a positive integer" in result[0].text
