# Verifying releases

Every release ships checksums, SBOMs, SLSA provenance, and a cosign-signed container image. This page gives the copy-paste commands to check each one. Substitute your version for `v0.2.0`. For how these artifacts get built and published, see [release-process.md](./release-process.md).

Tools you may need: [cosign](https://docs.sigstore.dev/cosign/system_config/installation/) and [slsa-verifier](https://github.com/slsa-framework/slsa-verifier#installation).

## Checksums

Each asset has a `.sha256` file, and `checksums.txt` covers the whole set:

```bash
# Everything you downloaded into the current directory:
sha256sum -c checksums.txt --ignore-missing

# A single file:
sha256sum -c linodemcp-linux-amd64.tar.gz.sha256
```

On macOS use `shasum -a 256 -c` in place of `sha256sum -c`.

## Container signature

Images are signed with cosign in keyless mode through GitHub's OIDC, so there is no maintainer-held key to leak. Verification proves the image was built and signed inside this repository's GitHub Actions:

```bash
cosign verify \
  --certificate-identity-regexp "https://github.com/chadit/LinodeMCP/.+" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/chadit/linodemcp:v0.2.0
```

A passing check prints the verified claims as JSON. Prefer the digest form (`ghcr.io/chadit/linodemcp@sha256:...`) when you want to pin exactly what you verified; the digest is shown by `docker buildx imagetools inspect ghcr.io/chadit/linodemcp:v0.2.0`.

## Binary provenance

`linodemcp.intoto.jsonl` on the release is SLSA Build Level 3 provenance covering every asset in `checksums.txt`:

```bash
slsa-verifier verify-artifact linodemcp-linux-amd64.tar.gz \
  --provenance-path linodemcp.intoto.jsonl \
  --source-uri github.com/chadit/LinodeMCP \
  --source-tag v0.2.0
```

This proves the file was built from this repository at that tag by the pinned SLSA builder, not assembled somewhere else.

## Container provenance

The image's provenance lives in the registry next to the image:

```bash
slsa-verifier verify-image \
  ghcr.io/chadit/linodemcp@sha256:DIGEST \
  --source-uri github.com/chadit/LinodeMCP \
  --source-tag v0.2.0
```

cosign can check the same attestation if you already have it installed:

```bash
cosign verify-attestation --type slsaprovenance \
  --certificate-identity-regexp "https://github.com/slsa-framework/slsa-github-generator/.+" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/chadit/linodemcp@sha256:DIGEST
```

## SBOMs

Each binary's SBOM is on the release in both formats (`linodemcp-<os>-<arch>.cdx.json` and `.spdx.json`), as are the image's (`linodemcp-image.cdx.json`, `linodemcp-image.spdx.json`). The image's SPDX SBOM is also attached in the registry:

```bash
cosign download sbom ghcr.io/chadit/linodemcp:v0.2.0
```

SBOMs for Go binaries are read from the embedded module info: exact for direct dependencies, best-effort for indirect ones.
