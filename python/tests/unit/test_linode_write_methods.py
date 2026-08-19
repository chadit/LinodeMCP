"""Behavioral tests for previously-untested Client write methods.

These cover the struct-returning write methods (create/update/resize/clone/boot)
that decode the API body through the `_parse_*` helpers and wrap httpx failures
in NetworkError. The proto `*_raw` siblings were already tested; the typed
methods here were whole untested branches.
"""

import pytest

pytestmark = pytest.mark.asyncio
