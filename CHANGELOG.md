# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0]

First release.

### Added

- **Cross-agent ingestion** for Claude Code, Codex CLI, Cursor CLI, OpenCode,
  Gemini CLI, Grok CLI, Qwen Code, Kimi Code, GitHub Copilot CLI, Cline, and Pi,
  normalized into one session shape.
- **Secret redaction and harness-noise removal** at the read boundary, including
  provider-shaped keys, assignment-shaped secrets, JWTs, private key blocks, and
  absolute home paths.
- **Task-arc segmentation** with idle splitting and continuation folding.
- **Deterministic clustering** blending prompt text with step sequences, plus a
  centroid merge pass.
- **Canonical step sequences** with per-step support, and cadence measured in
  distinct days rather than runs.
- **Correction mining**: standing preferences grouped across sessions into rule
  candidates.
- **Explainable scoring** across nine components with named penalties.
- **Classification** into skill, command, rule, hook, reference, update, or
  ignore, including detection of workflows that already have a skill.
- **Artifact generation** in the open Agent Skills format, with evidence and
  provenance in every file.
- **Optional prose authoring** through a locally installed agent CLI, with
  validation that discards a drifting rewrite.
- **Install planner** across nine harnesses, with dry runs, duplicate-safe rule
  appending, and no silent overwrites.
- **Local dashboard** (`ritual ui`) on loopback behind a per-run token,
  read-only unless `--allow-install`.
- **Benchmark** (`ritual eval`) that uses hand-written skills as ground truth by
  re-mining only the sessions that predate each one.

[Unreleased]: https://github.com/HarjjotSinghh/ritual/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/HarjjotSinghh/ritual/releases/tag/v0.1.0
