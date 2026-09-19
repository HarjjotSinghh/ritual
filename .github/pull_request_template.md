## What this changes

<!-- One paragraph. If it adds an agent, say which and where its sessions live. -->

## Checks

- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] New behaviour has a test that would fail without the change
- [ ] No fixture contains a real transcript, path, or token
- [ ] Output stays deterministic: no map iteration reaching a report, no
      floating-point ordering without a lexical fallback
