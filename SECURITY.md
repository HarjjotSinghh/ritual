# Security

## Reporting

Report a vulnerability through
[GitHub Security Advisories](https://github.com/HarjjotSinghh/ritual/security/advisories/new)
rather than a public issue. Expect an acknowledgement within 72 hours.

## Threat model

`ritual` reads the most sensitive corpus on a developer's machine: every
transcript their coding agents have written. The security properties that follow
from that:

**No network egress.** The binary makes no outbound connections. The single
exception is `internal/authoring`, which executes an agent CLI already installed
on the machine, and sends it the finding — never a transcript.

**Redaction at the read boundary.** Secrets are stripped in `internal/ingest`
before anything else in the program sees them. This is pattern matching, so it
catches shaped credentials and misses unshaped ones; see
[docs/privacy.md](docs/privacy.md) for the explicit list of what it does not
cover.

**Owner-only artifacts.** `~/.ritual` is 0700; reports and generated artifacts
are 0600. Reports contain prompt excerpts.

**The dashboard is loopback-only and token-gated.** `ritual ui` binds to
127.0.0.1 with a per-run token that every route checks, including reads, because
any page in the user's browser can issue a cross-origin request. The served page
declares `default-src 'none'` and cannot reach the network. Installation from
the browser is refused unless the process was started with `--allow-install`.

**Installation never overwrites silently.** Agent skill and rules directories
hold work the operator wrote by hand. An existing file is skipped with a stated
reason unless `--force` is passed, and rules are appended with duplicate
detection rather than rewritten.

**Transcript content is data, never instruction.** `ritual` parses transcripts
that may contain text addressed to an agent. That text is treated as a string to
cluster, never as something to act on. The authoring step is the only place
transcript-derived text reaches a model, and it arrives as quoted evidence
inside a prompt whose output is validated before use.

## Out of scope

- Anything that requires an attacker to already have read access to the
  operator's home directory, since they would already have the transcripts.
- Redaction completeness. It is a filter. The documentation says so, and the
  tool prints reports the operator is expected to read before sharing.
