"""Networking tools for LinodeMCP."""

from __future__ import annotations

import re

# Lowercase region slug (mirrors Go's validRegionSlugRegex) used to reject a
# malformed region_id locally on vlan_delete instead of forwarding it.
_REGION_SLUG_RE = re.compile(r"^[a-z0-9][a-z0-9-]*[a-z0-9]$")
