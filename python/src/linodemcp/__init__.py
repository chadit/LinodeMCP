"""LinodeMCP - MCP server for Linode API access.

Importing this package registers buf's generated protovalidate messages under
the names protovalidate's runtime imports them by. That runtime does
`from buf.validate import validate_pb2` and the published protovalidate wheel
ships no such module, so a plain `pip install protovalidate` raises
ModuleNotFoundError; buf writes the same messages into `linodemcp.genpb.buf`
(see buf.gen.protovalidate.yaml). The registration sits in the package root
because it has to run before anything imports protovalidate, and importing the
package is the one thing every entry point into this codebase does first.
"""

import sys

# Alias the already-imported modules instead of letting a second import find
# the same files: the descriptor pool refuses one file twice.
from linodemcp.genpb.buf import validate as _buf_validate
from linodemcp.genpb.buf.validate import validate_pb2 as _validate_pb2

sys.modules.setdefault("buf", sys.modules["linodemcp.genpb.buf"])
sys.modules.setdefault("buf.validate", _buf_validate)
sys.modules.setdefault("buf.validate.validate_pb2", _validate_pb2)

__version__ = "0.1.0"
__all__ = ["__version__"]
