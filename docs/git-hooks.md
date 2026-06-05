# Git hooks

LinodeMCP uses pre-commit for both commit-time and push-time local checks.

Install both hook types from the repository root:

```bash
make install-hooks
```

The Go and Python Makefile bootstrap paths also call this shared installer, so
running normal development targets from either language directory repairs a
missing `pre-commit` or `pre-push` hook when pre-commit is available.

Before a worker handoff or direct push, verify the local hook policy with:

```bash
make check-hooks
```

`make check-hooks` fails if either generated hook file is absent or not managed
by pre-commit. This keeps the configured pre-push hooks active, including
`make-lint` and `make-build` from `.pre-commit-config.yaml`.
