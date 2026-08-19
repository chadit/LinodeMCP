# Object Storage data plane

Two tools move object bytes: `linode_object_storage_object_upload` sends a local
file into a bucket, and `linode_object_storage_object_download` fetches an
object back out to a local file. Copies and multipart are not built yet.

## How it works, and why no keys are involved

Each tool makes exactly one Linode API call:
`POST /object-storage/buckets/{region}/{label}/object-url`, the same presigned
URL endpoint `linode_object_storage_presigned_url_create` exposes. The API
answers with a signed URL, and the tool then sends the file's bytes to that URL
(upload) or reads them back from it (download).

Following a URL is not the same as calling an S3 endpoint. The server builds no
S3 path, resolves no S3 hostname, and never creates or holds an Object Storage
access key. If you are auditing what this tool can reach, the answer is one
documented v4 route plus whatever host Linode put in the URL it minted.

The URL is a bearer credential for its lifetime. Anyone holding it can write
that one object until it expires, which is why the tool never prints it, not in
a result, not in an error, and not in the audit record.

## What you need on the machine running the server

The server reads and writes the file itself, so the path you pass has to be on
the server's own filesystem. That works for a stdio deployment, where the server
runs beside you. It does not work for a remote HTTP server with no shared
filesystem: there is no upload channel from the client to the server, and adding
one would mean pushing the whole object through the model's context.

`source_path` and `dest_path` must be absolute. Whether a path is absolute is
decided by the text of the path rather than by asking the operating system, so
the same call is legal or refused the same way on every platform.

## Downloading

`linode_object_storage_object_download` writes the object to `dest_path` and
reports `size_bytes` and `etag`. It refuses a path that already holds a file
unless you pass `overwrite: true`, and it makes no API call at all when it
refuses, so a mistyped destination costs nothing.

There is no confirm gate. The tool changes nothing on Linode, so a confirmation
prompt would announce a mutation that never happens; `overwrite` defaulting to
false is what guards the one thing it does change, which is a file on your disk.

The bytes land in a temporary file beside the destination and are renamed into
place only after the whole object has been written. A transfer that fails
partway leaves no truncated file at the path you named.

## Verifying an upload

The result carries `size_bytes`, `etag`, and `upload_mode`.

For a single-part upload, which is the only kind this tool performs today, the
endpoint's ETag is the MD5 of the stored object. To confirm the bytes arrived
intact, hash your local file and compare:

```sh
md5sum /srv/data/app.tar.gz    # or `md5 -q` on macOS
```

That comparison is worth more than anything the server could report back on its
own, because it checks the round trip against a value you computed yourself
rather than restating the server's arithmetic.

`upload_mode` always reads `single` right now. It exists so the answer does not
change shape when multipart lands, and so a future composite ETag is never
mistaken for a content hash: a multipart ETag is the hash of concatenated part
hashes, and comparing it against your file's MD5 would fail even on a perfect
upload.

An empty `etag` means the endpoint sent none. The object was stored, but nothing
came back to verify it against.

## The single-part ceiling

One presigned PUT carries at most `maxSinglePartBytes`, which defaults to 5 GiB.
A larger file is refused before any API call, naming both the ceiling and the
size you have. Multipart upload is what carries a larger object and it is not
implemented, so the refusal is the honest answer rather than a truncated object.

## Configuration

All four settings live under `objectStorage` and all are optional:

```yaml
objectStorage:
  # Confine uploads to a directory tree. Unset by default, which confines
  # nothing: a stdio server reads the operator's own paths.
  filesystemRoot: /srv/exports
  # Largest file one presigned PUT carries. Default 5 GiB.
  maxSinglePartBytes: 5368709120
  # Budget for one whole transfer, not one attempt. Default 30m.
  transferTimeout: 30m
  # Lifetime requested for the minted URL. Default 3600.
  presignTtlSeconds: 3600
```

`filesystemRoot` has no default because there is no safe directory to guess for
a server reading an operator's own files. When you do set one, a path is
resolved through symlinks before it is compared, so a link inside the root that
points outside it is refused rather than followed. It confines downloads too: a
`dest_path` outside the root is refused before any call goes out.

Neither transfer is retried. Each consumes its stream as it goes, so a replay
after a timeout would resume against bytes the first attempt already drained.

## Previewing

`dry_run: true` reports the presign request the tool would make and the number
of bytes it would send. It stats the file but opens nothing and calls nothing,
and it tells you up front when the file is already over the ceiling.
