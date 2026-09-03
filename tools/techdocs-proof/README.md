# techdocs-proof

Compares the rendered Linode TechDocs contract against this repository's proto
contract and reports where they disagree. It is a tool project: never installed
with the server, never imported by it, and carrying no runtime dependencies
beyond the `buf` and `gh` binaries it shells out to.

Start with [docs/README.md](./docs/README.md). The nine chapters under `docs/`
are the long form; chapter 01 states the authority boundary the whole thing
serves, and chapter 08 is the operations page.

## Running it

Scraping needs the network, so it runs from a schedule
(`.github/workflows/techdocs-drift.yml`) or by hand, never from `make check`:

```bash
PYTHONPATH=tools/techdocs-proof/src python3 -m techdocs_proof
```

The proto side comes from the working tree this command lives in.
`--github-source` swaps that for an immutable commit archive resolved with
`gh`, which is what a run needs when it must name a SHA.

Evidence never lands in the repository. Dated runs and `latest.json` go to
`$LINODEMCP_TECHDOCS_PROOF_ROOT`, else `$XDG_DATA_HOME/linodemcp/techdocs-proof`,
else `~/.local/share/linodemcp/techdocs-proof`; `--evidence-root` overrides it
and a root inside the repository is refused by name. `--retention-days` sweeps
dated runs older than five days by default.

The offline arm is what `make check` runs:

```bash
make techdocs-proof
```

## The route snapshot

Every run writes `route-snapshot.txt` beside its other evidence: one line per
route the rendered TechDocs state, with the route's API surface and whether the
site marks it deprecated. That file is the refresh candidate for
`docs/contracts/api-techdocs-routes-baseline.txt`, which
`scripts/verify_techdocs_routes.py` gates `proto/` against offline in
`make check`.

The scheduled workflow diffs the candidate against the reviewed file and reports
the difference in its job summary. It does not land it: REQ-D5 keeps every
repository change behind a human reviewing a diff. To write one from a run by
hand:

```bash
PYTHONPATH=tools/techdocs-proof/src python3 -m techdocs_proof \
  --techdocs-contract <run>/techdocs-contracts.json \
  --emit-route-snapshot docs/contracts/api-techdocs-routes-baseline.txt
```

That mode reads a contract and writes a file. It never scrapes.

## What lives here

- `src/techdocs_proof/proof.py` is the comparator, including `--self-test`.
- `data/known-divergences.json` is the exclusion ledger: divergences a triage
  ruled are not repo defects, each scoped to one route, location, and
  parameter, each carrying its reason. The self-test refuses a duplicate, a
  malformed entry, or a kind the comparison cannot raise, so a bad edit fails
  the gate rather than silently widening what counts as accepted.

  An entry takes one of two forms. The per-key form carries `category`, `kind`,
  `method`, `shape`, `location`, `parameter` and `reason`. The class form carries
  `category`, `kind`, `reason` and an `entries` list of `method`, `shape`,
  `location` and `parameter`, which is one approval item over many keys where
  writing the same reason out forty times would be the only difference.

  A class rule writes its keys out. There is no pattern form, and that is the
  point: a pattern absorbs whatever starts matching it later and can never go
  stale, which makes it a suppression rather than a ledger entry. A class rule
  expands into per-key entries at load, so a key that stops appearing in a run
  is reported in `known_divergences_unmatched` exactly as a per-key entry is.
- `docs/` is the wiki.
