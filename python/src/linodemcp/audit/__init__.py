"""Audit event schema and supporting helpers.

Mirrors ``go/internal/audit``. Phase 1a (this code) builds the event
dataclass and the redaction helper; Phase 1b wires capture into the
tool dispatch middleware; later phases add JSONL and SQLite sinks
plus query tools.
"""

from __future__ import annotations

from linodemcp.audit.event import (
    EVENT_ID_PREFIX,
    Capability,
    Event,
    Mode,
    Status,
    event_timestamp,
    finalize,
    new_event,
    new_event_id,
    set_mode,
)
from linodemcp.audit.export import (
    DEFAULT_EXPORT_MAX_RECORDS,
    EXPORT_FORMAT_CSV,
    EXPORT_FORMAT_JSON,
    EXPORT_FORMAT_NDJSON,
    MAX_EXPORT_RECORDS,
    UnknownExportFormatError,
    encode_events,
    export_events,
    export_to_file,
    resolve_max_records,
)
from linodemcp.audit.health import (
    health,
)
from linodemcp.audit.jsonl import (
    ACTIVE_LOG_FILE_NAME,
    JSONLSink,
    JSONLSinkClosedError,
)
from linodemcp.audit.path import (
    SYSTEM_AUDIT_DIR,
    USER_AUDIT_DIR_RELATIVE,
    resolve_default_audit_dir,
)
from linodemcp.audit.reader import (
    DEFAULT_RECENT_LIMIT,
    MAX_RECENT_LIMIT,
    RecentQuery,
    read_recent,
)
from linodemcp.audit.redact import (
    REDACTED_VALUE,
    is_redacted,
    redact,
    redact_with_pii,
    redaction_field_set,
    redaction_field_set_pii,
    redaction_fields,
    redaction_fields_pii,
)
from linodemcp.audit.report import (
    ReportDefinitionError,
    report,
)
from linodemcp.audit.retention import (
    DEFAULT_AUDIT_RETENTION_DAYS,
    DEFAULT_RETENTION_SWEEP_INTERVAL_SECONDS,
    RetentionSweeper,
)
from linodemcp.audit.sink import CapturingSink, MultiSink, NoopSink, Sink
from linodemcp.audit.sqlite import SQLiteSink
from linodemcp.audit.store import (
    read_with_fallback,
    resolve_sqlite_path,
    store_warning,
)
from linodemcp.audit.summary import (
    SummaryQuery,
    UnknownGroupByColumnError,
    load_window,
    summarize,
    summary_over,
    validate_group_by,
)

__all__ = [
    "ACTIVE_LOG_FILE_NAME",
    "DEFAULT_AUDIT_RETENTION_DAYS",
    "DEFAULT_EXPORT_MAX_RECORDS",
    "DEFAULT_RECENT_LIMIT",
    "DEFAULT_RETENTION_SWEEP_INTERVAL_SECONDS",
    "EVENT_ID_PREFIX",
    "EXPORT_FORMAT_CSV",
    "EXPORT_FORMAT_JSON",
    "EXPORT_FORMAT_NDJSON",
    "MAX_EXPORT_RECORDS",
    "MAX_RECENT_LIMIT",
    "REDACTED_VALUE",
    "SYSTEM_AUDIT_DIR",
    "USER_AUDIT_DIR_RELATIVE",
    "Capability",
    "CapturingSink",
    "Event",
    "JSONLSink",
    "JSONLSinkClosedError",
    "Mode",
    "MultiSink",
    "NoopSink",
    "RecentQuery",
    "ReportDefinitionError",
    "RetentionSweeper",
    "SQLiteSink",
    "Sink",
    "Status",
    "SummaryQuery",
    "UnknownExportFormatError",
    "UnknownGroupByColumnError",
    "encode_events",
    "event_timestamp",
    "export_events",
    "export_to_file",
    "finalize",
    "health",
    "is_redacted",
    "load_window",
    "new_event",
    "new_event_id",
    "read_recent",
    "read_with_fallback",
    "redact",
    "redact_with_pii",
    "redaction_field_set",
    "redaction_field_set_pii",
    "redaction_fields",
    "redaction_fields_pii",
    "report",
    "resolve_default_audit_dir",
    "resolve_max_records",
    "resolve_sqlite_path",
    "set_mode",
    "store_warning",
    "summarize",
    "summary_over",
    "validate_group_by",
]
