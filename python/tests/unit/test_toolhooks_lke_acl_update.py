"""The preview hook linode_lke_acl_update owns.

The Go twin is ``lke_acl_update_test.go`` in ``go/internal/toolhooks``, and
every sentence below is one both languages answer.
"""

from __future__ import annotations

from typing import Any

import pytest

_ACL_PATH = "/lke/clusters/123/control_plane_acl"
_CURRENT: dict[str, Any] = {"enabled": False, "addresses": {"ipv4": [], "ipv6": []}}

pytestmark = pytest.mark.asyncio
