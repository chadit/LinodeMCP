"""Histogram bucket boundaries must match the shared cross-language fixture.

testdata/observability/duration_buckets.json pins the bucket boundaries for
both duration instruments; Go's duration_buckets_test.go asserts the same
values on its side. The exported Prometheus _bucket series derive from these
boundaries, so a one-sided edit forks the metrics contract; this test makes
it fail here instead.

Kept separate from test_observability.py, which replaces the opentelemetry
modules with mocks at import time; this test needs the real module constants.
"""

from __future__ import annotations

import json
from pathlib import Path

from linodemcp.observability import (
    API_REQUEST_DURATION_BOUNDARIES,
    REQUEST_DURATION_BOUNDARIES,
)

_FIXTURE = (
    Path(__file__).resolve().parents[3]
    / "testdata"
    / "observability"
    / "duration_buckets.json"
)


def test_duration_buckets_match_shared_fixture() -> None:
    fixture = json.loads(_FIXTURE.read_text(encoding="utf-8"))

    assert (
        list(REQUEST_DURATION_BOUNDARIES)
        == (fixture["linodemcp.request.duration.seconds"])
    )
    assert (
        list(API_REQUEST_DURATION_BOUNDARIES)
        == (fixture["linodemcp.api.request.duration.seconds"])
    )


def test_boundaries_are_strictly_increasing() -> None:
    """A misordered boundary list would export nonsense buckets."""
    for boundaries in (REQUEST_DURATION_BOUNDARIES, API_REQUEST_DURATION_BOUNDARIES):
        assert list(boundaries) == sorted(boundaries)
        assert len(set(boundaries)) == len(boundaries)
