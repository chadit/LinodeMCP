"""Object Storage config block tests.

Mirrors ``go/internal/config/object_storage_config_test.go``. Covers the
presign lifetime window config load holds ``presignTtlSeconds`` to, and the
explicit zero both languages read as unset.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

import pytest

from linodemcp.config import (
    DEFAULT_PRESIGN_TTL_SECONDS,
    ConfigInvalidError,
    load_from_file,
)

if TYPE_CHECKING:
    from pathlib import Path

_MINIMAL = """
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
"""


def _write(tmp_path: Path, object_storage_block: str) -> Path:
    """Write a minimal config with the supplied objectStorage block appended."""
    path = tmp_path / "config.yml"
    path.write_text(_MINIMAL + object_storage_block, encoding="utf-8")
    return path


@pytest.mark.parametrize(
    ("block", "want"),
    [
        ("", DEFAULT_PRESIGN_TTL_SECONDS),
        ("objectStorage:\n  presignTtlSeconds: 0\n", DEFAULT_PRESIGN_TTL_SECONDS),
        ("objectStorage:\n  presignTtlSeconds: 900\n", 900),
    ],
)
def test_presign_ttl_accepted(tmp_path: Path, block: str, want: int) -> None:
    """An omitted key and an explicit 0 both resolve to the default.

    Go's setObjectStorageDefaults reads 0 as unset the same way, so one config
    file loads identically in both binaries.
    """
    cfg = load_from_file(_write(tmp_path, block))

    assert cfg.object_storage.presign_ttl_seconds == want


@pytest.mark.parametrize(
    "block",
    [
        "objectStorage:\n  presignTtlSeconds: 359\n",
        "objectStorage:\n  presignTtlSeconds: 3601\n",
    ],
)
def test_presign_ttl_outside_the_window_is_refused(tmp_path: Path, block: str) -> None:
    """The value travels as the object-url route's expires_in, which refuses
    anything outside 360..3600, so config load refuses it first.
    """
    with pytest.raises(ConfigInvalidError, match="presignTtlSeconds must be between"):
        load_from_file(_write(tmp_path, block))


@pytest.mark.parametrize(
    "block",
    [
        'objectStorage:\n  presignTtlSeconds: "900"\n',
        "objectStorage:\n  presignTtlSeconds: true\n",
        "objectStorage:\n  presignTtlSeconds: [900]\n",
    ],
)
def test_presign_ttl_of_the_wrong_type_is_refused(tmp_path: Path, block: str) -> None:
    """Go's yaml decode into an int field refuses these outright, so Python
    owes a config error rather than the TypeError the bound comparison raises.
    """
    with pytest.raises(
        ConfigInvalidError, match="presignTtlSeconds must be a whole number"
    ):
        load_from_file(_write(tmp_path, block))
