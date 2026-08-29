"""Preview branches whose earlier tests rode the removed typed client methods.

Each test drives the public hook with the live route-era seams, pinning the
sentence the preview reports rather than the client method it once mocked.
"""

from __future__ import annotations

import pytest

pytestmark = pytest.mark.asyncio
