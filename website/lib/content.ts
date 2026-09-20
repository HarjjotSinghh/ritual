import {
  BookMarked,
  Download,
  Eye,
  FileSearch,
  FilePenLine,
  FlaskConical,
  Hammer,
  LayoutDashboard,
  type LucideIcon,
  Network,
  Package,
  Route,
  Scale,
  Search,
  Terminal,
  Webhook,
  Workflow,
} from "lucide-react";

import type { BrandKey } from "@/lib/brand-marks";

export const REPO = "https://github.com/HarjjotSinghh/ritual";

export const INSTALL_COMMAND =
  "go install github.com/HarjjotSinghh/ritual/cmd/ritual@latest";

export type ConsoleLine = {
  text: string;
  tone?: "dim" | "accent" | "good" | "warn" | "bold";
};

/** Example `ritual scan` output. Shaped like the real thing, not a mockup. */
export const scanOutput: ConsoleLine[] = [
  { text: "" },
  { text: "Scanned", tone: "bold" },
  {
    text: "  claude       211 sessions   21200 turns    9123 tool calls  2026-07-02 > 2026-09-19",
  },
  {
    text: "  codex         39 sessions    2841 turns    1204 tool calls  2026-07-14 > 2026-09-18",
  },
  {
    text: "  cursor        25 sessions    5016 turns    3676 tool calls  2026-08-02 > 2026-09-15",
  },
  {
    text: "  opencode      10 sessions    1636 turns     791 tool calls  2026-09-01 > 2026-09-17",
  },
  {
    text: "  831 task arcs, 215 of them part of a repeating workflow, in 9.8s",
    tone: "dim",
  },
  { text: "" },
  { text: "Found 6 recurring workflows", tone: "bold" },
  { text: "  id         score  kind      runs  days  workflow", tone: "dim" },
  {
    text: "  360119359c   71   skill       14     9  Verify the storefront after a theme push",
    tone: "good",
  },
  {
    text: "             Ran 14 times across 12 sessions in storefront, about daily.",
    tone: "dim",
  },
  {
    text: "  0248c624a4   64   skill        9     9  Draft the end-of-day update",
    tone: "good",
  },
  {
    text: "  0452ba80b6   58   hook         7     6  Run the regression pass after a push",
    tone: "warn",
  },
  {
    text: "  479d0ebd2b   44   command      6     5  Summarise the open pull requests",
    tone: "accent",
  },
  {
    text: "  0c5ff3cbbe   31   reference    5     4  Where the billing webhooks are wired",
    tone: "dim",
  },
  {
    text: "  c54cf75eea   12   ignore       9     2  Read a file, then read another file",
    tone: "dim",
  },
  { text: "" },
  {
    text: "  ritual show 360119359c    to read the evidence behind a finding",
    tone: "dim",
  },
  {
    text: "  ritual install 360119359c to write it into your agents",
    tone: "dim",
  },
];

/** Example `ritual show <id>` output. */
export const evidenceOutput: ConsoleLine[] = [
  { text: "" },
  { text: "Verify the storefront after a theme push", tone: "bold" },
  { text: "  skill  score 71/100  cohesion 82%", tone: "dim" },
  { text: "" },
  { text: "Canonical sequence", tone: "bold" },
  { text: "  1. Run `shopify theme push`            14/14 runs" },
  { text: "  2. Open the storefront in a browser    13/14 runs" },
  { text: "  3. Check the cart and checkout flow    12/14 runs" },
  { text: "  4. Read the browser console             9/14 runs", tone: "dim" },
  { text: "" },
  { text: "Corrections you had to repeat", tone: "bold" },
  { text: '  "no, also check it on mobile"          4 sessions', tone: "warn" },
  { text: '  "use the staging password first"       3 sessions', tone: "warn" },
  { text: "" },
  { text: "Evidence", tone: "bold" },
  {
    text: "  2026-09-18  claude  ${HOME}/.claude/projects/storefront/4f2c.jsonl",
    tone: "dim",
  },
  {
    text: "  2026-09-16  cursor  ${HOME}/.cursor/projects/storefront/a81e.jsonl",
    tone: "dim",
  },
  {
    text: "  2026-09-11  claude  ${HOME}/.claude/projects/storefront/91bd.jsonl",
    tone: "dim",
  },
  { text: "  ... 11 more across 12 sessions", tone: "dim" },
];

/**
 * The six verdicts that produce an artifact. `ignore` is the seventh and is
 * described in the note under the grid rather than given a cell of its own:
 * it is the verdict that deliberately writes nothing, so a card for it would
 * be an empty promise in a grid of things you can install.
 */
export const verdicts: {
  kind: string;
  when: string;
  why: string;
  Icon: LucideIcon;
}[] = [
  {
    kind: "skill",
    Icon: Workflow,
    when: "A multi-step procedure with judgement in it.",
    why: "Where your undocumented decisions live.",
  },
  {
    kind: "command",
    Icon: Terminal,
    when: "Two or three steps, run on demand.",
    why: "A skill would be ceremony around two actions.",
  },
  {
    kind: "rule",
    Icon: Scale,
    when: "A standing preference: always, never, from now on.",
    why: "A preference you have to invoke is one you will forget.",
  },
  {
    kind: "hook",
    Icon: Webhook,
    when: "Work that always follows an event.",
    why: "You should not have to remember it at all.",
  },
  {
    kind: "reference",
    Icon: BookMarked,
    when: "The agent keeps rediscovering the same facts.",
    why: "Write the answer down once.",
  },
  {
    kind: "update",
    Icon: FilePenLine,
    when: "You already have a skill for this.",
    why: "Adding a second one is how skill directories rot.",
  },
];

export const agents: {
  name: string;
  path: string;
  format: string;
  icon: BrandKey;
}[] = [
  { name: "Claude Code", icon: "claude", path: "~/.claude/projects/<slug>/*.jsonl", format: "JSONL" },
  { name: "Codex CLI", icon: "codex", path: "~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl", format: "JSONL" },
  { name: "Cursor CLI", icon: "cursor", path: "~/.cursor/projects/<slug>/agent-transcripts/**", format: "JSONL" },
  { name: "OpenCode", icon: "opencode", path: "~/.local/share/opencode/opencode.db", format: "SQLite" },
  { name: "Gemini CLI", icon: "gemini", path: "~/.gemini/tmp/<hash>/chats/*.json", format: "JSON" },
  { name: "Grok CLI", icon: "grok", path: "~/.grok/sessions/<cwd>/<id>/chat_history.jsonl", format: "JSONL" },
  { name: "Qwen Code", icon: "qwen", path: "~/.qwen/projects/<slug>/chats/*.jsonl", format: "JSONL" },
  { name: "Kimi Code", icon: "kimi", path: "~/.kimi-code/sessions/**/state.json", format: "JSON" },
  { name: "Copilot CLI", icon: "copilot", path: "~/.copilot/session-state/**/events.jsonl", format: "JSONL" },
  { name: "Cline", icon: "cline", path: "~/.cline/tasks/<id>/api_conversation_history.json", format: "JSON" },
  { name: "Pi", icon: "pi", path: "~/.pi/agent/sessions/<slug>/*.jsonl", format: "JSONL" },
];

export const commands: {
  command: string;
  description: string;
  Icon: LucideIcon;
}[] = [
  { command: "ritual scan", Icon: Search, description: "Read your history and rank what it finds" },
  { command: "ritual show <id>", Icon: Eye, description: "Every run a finding came from, with transcript paths" },
  { command: "ritual build <id>", Icon: Hammer, description: "Write the artifact to ~/.ritual/out" },
  { command: "ritual install <id>", Icon: Download, description: "Install it into your agents" },
  { command: "ritual rules", Icon: Scale, description: "The preferences you keep restating" },
  { command: "ritual ui", Icon: LayoutDashboard, description: "The same report in a browser, served from localhost" },
  { command: "ritual agents", Icon: Network, description: "Where ritual looked, and what it found" },
  { command: "ritual eval", Icon: FlaskConical, description: "Would it have found the skills you wrote by hand?" },
];

export const installOptions: {
  id: string;
  label: string;
  command: string;
  note: string;
  /** A brand mark where one exists. Scoop publishes no vector logo, so that
      tab falls back to a generic package glyph rather than a wrong mark. */
  brand?: BrandKey;
  Icon?: LucideIcon;
}[] = [
  {
    id: "go",
    label: "Go",
    brand: "go",
    command: INSTALL_COMMAND,
    note: "Builds from source. Needs Go 1.26 or newer.",
  },
  {
    id: "binary",
    label: "Release",
    brand: "github",
    command: "open https://github.com/HarjjotSinghh/ritual/releases/latest",
    note: "Archives for macOS, Linux, and Windows on amd64 and arm64, with checksums.",
  },
  {
    id: "homebrew",
    label: "Homebrew",
    brand: "homebrew",
    command: "brew install --cask HarjjotSinghh/tap/ritual",
    note: "macOS. Published by the release workflow once the tap token is configured.",
  },
  {
    id: "scoop",
    label: "Scoop",
    Icon: Package,
    command: "scoop install ritual",
    note: "Windows. Add the bucket first: scoop bucket add harjjotsinghh https://github.com/HarjjotSinghh/scoop-bucket",
  },
];

/**
 * The three claims the rest of the page has to earn. Written as a run, not as
 * three matching cards, because they are not peers: the first is scope, the
 * second is the hard part, the third is why you should believe either.
 */
export const claims: {
  n: string;
  title: string;
  body: string;
  Icon: LucideIcon;
}[] = [
  {
    n: "01",
    title: "Every agent, not one",
    Icon: Network,
    body: "Eleven agents, one normalized shape. A workflow you do in Cursor on Monday and in Codex on Thursday is one workflow, and only a tool that reads both can see it.",
  },
  {
    n: "02",
    title: "Where it belongs, not just that it exists",
    Icon: Route,
    body: "A repeated behaviour can be six different things. Putting it in the wrong one is worse than leaving it alone: a standing preference written as a skill never fires.",
  },
  {
    n: "03",
    title: "Receipts on every claim",
    Icon: FileSearch,
    body: "How many separate days it happened on, which sessions it came from, which steps appeared in what share of runs, and what the score is made of.",
  },
];

export const faq = [
  {
    q: "Does anything get uploaded?",
    a: "No. There is no account, no telemetry, and no network call in the scan path. ritual is a single binary that reads local files and writes local files. The optional prose step shells out to an agent CLI you already installed, and sends it only the finding, never the transcripts.",
  },
  {
    q: "Will there be a hosted version?",
    a: "No. Asking a developer to upload six months of agent transcripts to a third party is asking them to upload their employer's source code and their production credentials. No privacy policy makes that reasonable to ask.",
  },
  {
    q: "How is this different from the other skill generators?",
    a: "Most of them read Claude Code only, ask a model what your skills should be, and emit a SKILL.md. ritual reads eleven agents, decides which of seven artifact kinds a pattern belongs in, and shows you the transcripts behind every suggestion so you can disagree with it.",
  },
  {
    q: "What if my agent stores history somewhere else?",
    a: "Run `ritual agents` to see every path it checked and what it found at each one. Storage layouts move; one file has to change when they do, and a pull request adding an agent is about forty lines.",
  },
  {
    q: "Does it overwrite skills I already wrote?",
    a: "No. A pattern that matches an existing skill is classified as an update, and `ritual install` writes nothing until you pass the id. Run it without --apply to see the exact file writes first.",
  },
  {
    q: "How much history does it need?",
    a: "Enough for a pattern to repeat on separate days. A workflow needs several runs across more than one session before it scores high enough to surface, which is why the ranking is stable rather than noisy.",
  },
] as const;
