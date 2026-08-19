# linodemcp

The Python implementation of LinodeMCP: an MCP server and CLI for the Linode API. The tool surface is generated from one protobuf contract shared with the Go implementation, so both languages answer identically.

## Install

```bash
pip install linodemcp
```

## Run

```bash
# MCP server on stdio
linodemcp

# CLI mode
linodemcp call linode_instance_list
```

Set `LINODE_TOKEN` (or configure a profile) before calling tools that reach the API.

Full documentation, the tool catalog, and the cross-language design live in the [repository README](https://github.com/chadit/LinodeMCP).
