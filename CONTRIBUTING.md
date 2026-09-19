# Contributing

The most useful contribution is an agent. The second most useful is a bug report
with the transcript shape that broke a reader.

## Adding an agent

About forty lines, in four places.

### 1. Describe where it keeps sessions — `internal/agentspec/spec.go`

```go
{
    Key: "yourAgent", DisplayName: "Your Agent", Vendor: "Somebody",
    RootEnv: "YOURAGENT_HOME",
    Roots:   []string{j(".youragent")},
    Marker:  "sessions", Glob: "sessions/**/*.jsonl", Layout: LayoutYourAgent,
    Excluded: []string{"auth.json", "cache"},
},
```

`Marker` is a child directory that must exist for the root to count — it stops
an empty config directory from claiming sessions it does not have. `Excluded`
keeps credential files and large install trees out of the walk; be generous,
because a walk that spends its budget on a bundled Chromium never reaches the
transcripts.

If the store is one file holding every session (a SQLite database), set
`WholeStore: true` so the per-file size limit does not skip it.

### 2. Write the reader — `internal/ingest/youragent.go`

```go
func readYourAgent(path string, lim Limits, red *redact.Redactor) ([]session.Session, error)
```

Return one `session.Session` per conversation, with turns built through
`turnBuilder` so redaction and truncation are applied for you. Register it in
`readerFor`.

Be lenient. A malformed line is skipped and counted, never fatal: a crashed
agent leaves a half-written last line in almost every store. If the records look
like the shared message shape — a role plus `content` or `parts` — call
`appendGenericMessage` and you are mostly done.

Run every human prompt through `CleanPrompt`, which strips harness scaffolding.
If your agent injects a wrapper this build does not know, add its tag to
`harnessTags` rather than special-casing it in the reader.

### 3. Add a fixture and a test

Put a small, hand-written transcript in `internal/ingest/testdata/`. Do not
paste a real one: it will contain paths, project names, and quite possibly a
token. Six to ten lines covering a prompt, a tool call, a tool result, an error,
and one malformed line is enough.

Then assert the things that actually break:

```go
func TestReadYourAgent(t *testing.T) {
    s := read(t, readYourAgent, "youragent.jsonl")
    // identity, prompts, tool calls with normalized commands, error results
}
```

### 4. Document it

Add a row to the storage table in `README.md`, and an install target in
`internal/install/install.go` if the agent reads skills.

## Ground rules

**Determinism is a correctness property.** The same history must always produce
the same report. No map iteration in output, no floating-point comparison
without a lexical fallback. There is a test for this; keep it passing.

**Redaction happens at the read boundary.** Never construct a string from
transcript content outside a reader without passing it through
`redact.Redactor`. Everything downstream assumes it is already clean.

**No network calls.** The only outbound path is `internal/authoring`, which
shells out to an agent CLI the operator already installed. Nothing else may talk
to anything.

**Say what you measured.** A step that appeared in two runs of three is written
as such, not smoothed into a certainty. A loose cluster says so. A score
publishes its components.

## Running things

```bash
make test      # the suite
make race      # under the race detector
make lint      # vet, gofmt, staticcheck when installed
make build     # dist/ritual
make snapshot  # multi-platform build via goreleaser
```

## Commit messages

Conventional commits. `feat(agent): add Windsurf` is ideal for a new adapter —
the release notes group those separately.
