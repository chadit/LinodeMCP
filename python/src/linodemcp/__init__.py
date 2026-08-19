"""LinodeMCP - MCP server for Linode API access."""

import sys

# protovalidate's runtime does `from buf.validate import validate_pb2`, and its
# published wheel does not ship that module: importing it after a plain
# `pip install protovalidate` raises ModuleNotFoundError. buf generates the same
# messages into the tree below (see buf.gen.protovalidate.yaml), so registering
# them under the name protovalidate looks for is what makes the library usable.
# Aliasing the already-imported module rather than letting a second import find
# the file is what keeps the descriptor pool from being handed one file twice.
#
# It sits in the package root because it has to run before anything imports
# protovalidate, and importing the package is the one thing every entry point
# into this codebase does first.
from linodemcp.genpb.buf import validate as _buf_validate
from linodemcp.genpb.buf.validate import validate_pb2 as _validate_pb2

sys.modules.setdefault("buf", sys.modules["linodemcp.genpb.buf"])
sys.modules.setdefault("buf.validate", _buf_validate)
sys.modules.setdefault("buf.validate.validate_pb2", _validate_pb2)

__version__ = "0.1.0"
__all__ = ["__version__"]
