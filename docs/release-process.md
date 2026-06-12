# Release process

Maintainer runbook for cutting a LinodeMCP release. Two workflows do the work:

- `release.yml` (manual): computes the next tag from a bump choice, generates the changelog, pushes the tag, and opens a draft GitHub release.
- `release-artifacts.yml` (called by the first, or triggered by a tag push): verifies both implementations, builds every artifact, signs the image, and attaches everything to the draft.

The maintainer's job is the first click and the last one: start the bump, then review and publish the draft.

## What a release ships

- Six Go binaries (`linux`, `darwin`, `windows` on `amd64` and `arm64`), packaged as `.tar.gz` (`.zip` for Windows), each with a `.sha256` file
- `checksums.txt` covering every asset
- Python wheel and sdist, stamped with the tag version, each with a `.sha256` file
- SBOMs per binary and for the image, in CycloneDX and SPDX JSON
- `linodemcp.intoto.jsonl`, the SLSA provenance for all release assets
- A multi-arch container image at `ghcr.io/chadit/linodemcp:<tag>`, signed with cosign (keyless), with its SPDX SBOM and SLSA provenance attached in the registry

## Cutting a stable release

1. Make sure `main` is green in CI.
2. Actions, "Release", "Run workflow". Pick the bump type. Use the `dryRun` input first if you want to preview the tag and notes without tagging.
3. The workflow pushes `vX.Y.Z` and opens a draft release with the generated changelog. The artifacts pipeline runs next; expect 10 to 20 minutes.
4. Review the draft: every asset from the list above is attached, the changelog reads well (add the hand-written summary paragraph at the top), and a spot check from [verifying-releases.md](verifying-releases.md) passes.
5. Publish the draft. The pipeline only moves the floating image tags (`X.Y`, `latest`) for stable tags, so publishing is the last gate.

## Cutting a pre-release

The bump workflow only produces plain `X.Y.Z` tags, so release candidates are tagged by hand:

```bash
git tag v0.2.0-rc1
git push origin v0.2.0-rc1
```

A manually pushed tag fires `release-artifacts.yml` directly. It generates the same changelog, creates the draft itself, marks it as a pre-release, and attaches the full artifact set. Floating image tags do not move for pre-releases. Review and publish as above.

## Re-running

- Whole pipeline for an existing tag: Actions, "Release Artifacts", "Run workflow", enter the tag. Asset uploads use `--clobber`, so re-runs replace cleanly.
- A single failed job from a transient runner issue: use "Re-run failed jobs" on the run page. Jobs are designed to be re-runnable.

## Version stamping

The tag is the single version for both implementations:

- Go binaries get `Version`, `commit`, and `buildDate` injected through ldflags into `internal/appinfo`.
- The Python wheel and sdist are built after stamping the tag version into the working copy's `pyproject.toml` and `version.py`. Nothing is committed.

Source defaults stay at their development values between releases; do not bump them as part of cutting a release.

## Failure modes

- **One platform fails to build.** The matrix is fail-fast, the publish job never runs, and no assets attach. Fix the cause and re-run via dispatch.
- **Image pushed but a later job failed.** The image tag exists in GHCR while the draft has no assets. Nothing user-visible happened (the release is still a draft); a re-run rebuilds and overwrites the image at the same tag and attaches the assets.
- **Artifact collection hiccup in publish.** The publish job pulls staged artifacts from the run with `gh run download`; if it flakes, re-run the publish job alone.

## First release

Expect the first `v0.2.0-rc1` to surface workflow bugs; that is what the rc is for. Fix what breaks, tag `rc2` if needed, then cut `v0.2.0`.
