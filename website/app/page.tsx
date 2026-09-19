import Link from "next/link";
import { ArrowUpRight } from "lucide-react";

import { Button } from "@/components/ui/button";
import { ThemeToggle } from "@/components/theme-toggle";
import { CopyCommand } from "@/components/copy-command";
import { InstallTabs } from "@/components/install-tabs";
import { Terminal } from "@/components/terminal";

const REPO = "https://github.com/HarjjotSinghh/ritual";

const scanOutput = [
  { text: "" },
  { text: "Scanned", tone: "bold" as const },
  { text: "  claude       211 sessions   21200 turns    9123 tool calls  2026-07-02 → 2026-09-19" },
  { text: "  codex         39 sessions    2841 turns    1204 tool calls  2026-07-14 → 2026-09-18" },
  { text: "  cursor        25 sessions    5016 turns    3676 tool calls  2026-08-02 → 2026-09-15" },
  { text: "  opencode      10 sessions    1636 turns     791 tool calls  2026-09-01 → 2026-09-17" },
  { text: "  831 task arcs, 215 of them part of a repeating workflow, in 9.8s", tone: "dim" as const },
  { text: "" },
  { text: "Found 6 recurring workflows", tone: "bold" as const },
  { text: "  id         score  kind      runs  days  workflow", tone: "dim" as const },
  { text: "  360119359c   71   skill       14     9  Verify the storefront after a theme push", tone: "good" as const },
  { text: "             Ran 14 times across 12 sessions in storefront, about daily.", tone: "dim" as const },
  { text: "  0248c624a4   64   skill        9     9  Draft the end-of-day update", tone: "good" as const },
  { text: "  0452ba80b6   58   hook         7     6  Run the regression pass after a push", tone: "warn" as const },
  { text: "  479d0ebd2b   44   command      6     5  Summarise the open pull requests", tone: "accent" as const },
  { text: "  0c5ff3cbbe   31   reference    5     4  Where the billing webhooks are wired", tone: "dim" as const },
  { text: "  c54cf75eea   12   ignore       9     2  Read a file, then read another file", tone: "dim" as const },
  { text: "" },
  { text: "Found 3 standing preferences you keep restating", tone: "bold" as const },
  { text: "  5154ac55f7  x4 Always check the mobile layout before reporting done." },
  { text: "  80800a871b  x3 Never edit the generated schema files directly." },
  { text: "  a91b2c3d4e  x2 Use the existing analytics event names; do not invent new ones." },
];

const evidenceOutput = [
  { text: "" },
  { text: "Verify the storefront after a theme push  (360119359c)", tone: "bold" as const },
  { text: "Ran 14 times across 12 sessions in storefront, about daily." },
  { text: "" },
  { text: "Canonical sequence", tone: "bold" as const },
  { text: "   1. Run `shopify theme push`                  ██████████ 14/14 runs" },
  { text: "   2. Verify in a browser                       ██████████ 14/14 runs" },
  { text: "   3. Run `npm run test:e2e`                    ████████·· 11/14 runs" },
  { text: "   4. Read the relevant files                   █████····· 7/14 runs" },
  { text: "" },
  { text: "Score breakdown", tone: "bold" as const },
  { text: "  recurrence   ██████████████████·· 0.91" },
  { text: "  complexity   ████████████████████ 1.00" },
  { text: "  repetition   ██████████████······ 0.71" },
  { text: "  friction     ████████············ 0.40" },
  { text: "" },
  { text: "Evidence", tone: "bold" as const },
  { text: "  2026-09-12 14:02 claude   verify the storefront theme after pushing live", tone: "dim" as const },
  { text: "    ${HOME}/.claude/projects/-storefront/9dd7cb3b.jsonl", tone: "dim" as const },
];

const verdicts = [
  {
    kind: "skill",
    when: "A multi-step procedure with judgement in it.",
    why: "Where your undocumented decisions live.",
  },
  {
    kind: "command",
    when: "Two or three steps, run on demand.",
    why: "A skill would be ceremony around two actions.",
  },
  {
    kind: "rule",
    when: "A standing preference: always, never, from now on.",
    why: "A preference you have to invoke is one you will forget.",
  },
  {
    kind: "hook",
    when: "Work that always follows an event.",
    why: "You should not have to remember it at all.",
  },
  {
    kind: "reference",
    when: "The agent keeps rediscovering the same facts.",
    why: "Write the answer down once.",
  },
  {
    kind: "update",
    when: "You already have a skill for this.",
    why: "Adding a second one is how skill directories rot.",
  },
  {
    kind: "ignore",
    when: "A real pattern, not worth an artifact.",
    why: "git status runs a hundred times a week.",
  },
];

const agents = [
  ["Claude Code", "~/.claude/projects/<slug>/*.jsonl", "JSONL"],
  ["Codex CLI", "~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl", "JSONL"],
  ["Cursor CLI", "~/.cursor/projects/<slug>/agent-transcripts/**", "JSONL"],
  ["OpenCode", "~/.local/share/opencode/opencode.db", "SQLite"],
  ["Gemini CLI", "~/.gemini/tmp/<hash>/chats/*.json", "JSON"],
  ["Grok CLI", "~/.grok/sessions/<cwd>/<id>/chat_history.jsonl", "JSONL"],
  ["Qwen Code", "~/.qwen/projects/<slug>/chats/*.jsonl", "JSONL"],
  ["Kimi Code", "~/.kimi-code/sessions/**/state.json", "JSON"],
  ["Copilot CLI", "~/.copilot/session-state/**/events.jsonl", "JSONL"],
  ["Cline", "~/.cline/tasks/<id>/api_conversation_history.json", "JSON"],
  ["Pi", "~/.pi/agent/sessions/<slug>/*.jsonl", "JSONL"],
];

const commands = [
  ["ritual scan", "Read your history and rank what it finds"],
  ["ritual show <id>", "Every run a finding came from, with transcript paths"],
  ["ritual build <id>", "Write the artifact to ~/.ritual/out"],
  ["ritual install <id>", "Install it into your agents"],
  ["ritual rules", "The preferences you keep restating"],
  ["ritual ui", "The same report in a browser, served from localhost"],
  ["ritual agents", "Where ritual looked, and what it found"],
  ["ritual eval", "Would it have found the skills you wrote by hand?"],
];

// lucide dropped brand marks in v1, and a single inline path is lighter than a
// second icon dependency for one logo.
function GitHubMark() {
  return (
    <svg viewBox="0 0 16 16" aria-hidden className="size-4" fill="currentColor">
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.42 7.42 0 0 1 2-.27c.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z" />
    </svg>
  );
}

export default function Home() {
  return (
    <>
      <header className="sticky top-0 z-10 border-b border-border bg-background/85 backdrop-blur">
        <nav className="mx-auto flex h-14 w-full max-w-5xl items-center gap-6 px-6">
          <span className="ui font-semibold tracking-tight">ritual</span>
          <div className="ui hidden items-center gap-5 text-sm text-muted-foreground sm:flex">
            <a href="#how" className="transition-colors hover:text-foreground">how</a>
            <a href="#privacy" className="transition-colors hover:text-foreground">privacy</a>
            <a href="#agents" className="transition-colors hover:text-foreground">agents</a>
            <a href="#install" className="transition-colors hover:text-foreground">install</a>
          </div>
          <div className="ml-auto flex items-center gap-1">
            <Button variant="ghost" size="icon-sm" render={<a href={REPO} aria-label="ritual on GitHub" />}>
              <GitHubMark />
            </Button>
            <ThemeToggle />
          </div>
        </nav>
      </header>

      <main className="mx-auto w-full max-w-5xl flex-1 px-6">
        {/* Hero */}
        <section className="py-20 md:py-28">
          <p className="ui mb-5 text-xs tracking-[0.18em] text-muted-foreground uppercase">
            Cross-agent process mining
          </p>
          <h1 className="max-w-3xl text-4xl leading-[1.08] md:text-6xl">
            Your coding agents have been writing down how you work.
          </h1>
          <p className="mt-6 max-w-2xl text-lg text-muted-foreground md:text-xl">
            Months of transcripts are sitting in <code className="text-foreground">~/.claude</code>,{" "}
            <code className="text-foreground">~/.codex</code>,{" "}
            <code className="text-foreground">~/.cursor</code> and half a dozen other
            directories right now. Every task you gave, every command that ran, every time you
            had to say <em>&ldquo;no, also check mobile&rdquo;</em> for the fourth time.
          </p>
          <p className="mt-4 max-w-2xl text-lg md:text-xl">
            That is a record of how you actually work, and nothing reads it. ritual does.
          </p>

          <div className="mt-9 max-w-xl">
            <CopyCommand command="go install github.com/HarjjotSinghh/ritual/cmd/ritual@latest" />
          </div>
          <div className="mt-4 flex flex-wrap items-center gap-3">
            <Button size="lg" render={<a href="#install" />}>
              Other ways to install
            </Button>
            <Button variant="ghost" size="lg" render={<a href={REPO} />}>
              Read the source
              <ArrowUpRight aria-hidden data-icon="inline-end" />
            </Button>
          </div>
        </section>

        {/* Demo */}
        <section className="rule py-14">
          <Terminal title="ritual scan" command="ritual scan" lines={scanOutput} />
          <p className="mt-4 text-sm text-muted-foreground">
            Example output. Yours will name your own work.
          </p>
        </section>

        {/* Not another skill generator */}
        <section id="how" className="rule scroll-mt-20 py-14">
          <h2 className="text-2xl md:text-3xl">Not another skill generator</h2>
          <p className="mt-4 max-w-2xl text-muted-foreground">
            Several tools turn Claude Code history into skills. ritual differs in three ways
            that matter.
          </p>

          <div className="mt-10 grid gap-10 md:grid-cols-3">
            <article>
              <h3 className="text-base">Every agent, not one</h3>
              <p className="mt-3 text-[15px] text-muted-foreground">
                Eleven agents, one normalized shape. A workflow you do in Cursor on Monday and
                in Codex on Thursday is one workflow, and only a tool that reads both can see
                it.
              </p>
            </article>
            <article>
              <h3 className="text-base">Where it belongs, not just that it exists</h3>
              <p className="mt-3 text-[15px] text-muted-foreground">
                A repeated behaviour can be six different things. Putting it in the wrong one
                is worse than leaving it alone: a standing preference written as a skill never
                fires.
              </p>
            </article>
            <article>
              <h3 className="text-base">Receipts on every claim</h3>
              <p className="mt-3 text-[15px] text-muted-foreground">
                How many separate days it happened on, which sessions it came from, which steps
                appeared in what share of runs, and what the score is made of.
              </p>
            </article>
          </div>

          <div className="mt-12 overflow-hidden rounded-xl border border-border">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-border bg-muted/40">
                <tr>
                  <th className="px-4 py-3 font-medium">verdict</th>
                  <th className="px-4 py-3 font-medium">when</th>
                  <th className="hidden px-4 py-3 font-medium sm:table-cell">why not a skill</th>
                </tr>
              </thead>
              <tbody>
                {verdicts.map((row) => (
                  <tr key={row.kind} className="border-b border-border last:border-0">
                    <td className="ui px-4 py-3 align-top text-primary">{row.kind}</td>
                    <td className="px-4 py-3 align-top">{row.when}</td>
                    <td className="hidden px-4 py-3 align-top text-muted-foreground sm:table-cell">
                      {row.why}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        {/* Evidence */}
        <section className="rule py-14">
          <h2 className="text-2xl md:text-3xl">No finding is a vibe</h2>
          <p className="mt-4 max-w-2xl text-muted-foreground">
            Every suggestion carries the transcripts it came from. Open one and check it — that
            is the whole point of mining evidence rather than asking a model what it thinks you
            do.
          </p>
          <Terminal
            className="mt-8"
            title="ritual show 360119359c"
            command="ritual show 360119359c"
            lines={evidenceOutput}
          />
        </section>

        {/* Privacy */}
        <section id="privacy" className="rule scroll-mt-20 py-14">
          <h2 className="text-2xl md:text-3xl">Nothing leaves your machine</h2>
          <div className="mt-6 grid gap-8 md:grid-cols-2">
            <div className="space-y-4 text-muted-foreground">
              <p>
                There is no account, no upload, and no telemetry — not as a setting you can turn
                off, but because the input is your entire working history. It contains client
                code, customer data, internal URLs, and every credential anyone ever pasted into
                a prompt.
              </p>
              <p>
                So ritual is a single binary that reads local files and writes local files.
                Secrets are stripped at the read boundary, home paths become{" "}
                <code className="text-foreground">${"{HOME}"}</code>, and the dashboard binds to
                loopback behind a token.
              </p>
              <p>
                The optional prose step shells out to an agent CLI you already installed and
                trusted, and sends it only the finding — never the transcripts.
              </p>
            </div>
            <div className="rounded-xl border border-border bg-card p-6">
              <h3 className="text-base">Why there is no hosted version</h3>
              <p className="mt-3 text-[15px] text-muted-foreground">
                The obvious product is a website: run a command, upload the output, get skills
                back. It will not be built.
              </p>
              <p className="mt-3 text-[15px] text-muted-foreground">
                Asking a developer to upload six months of agent transcripts to a third party is
                asking them to upload their employer&rsquo;s source code and their production
                credentials. No privacy policy makes that reasonable to ask.
              </p>
              <Button
                variant="outline"
                size="sm"
                className="mt-5"
                render={<a href={`${REPO}/blob/main/docs/privacy.md`} />}
              >
                Read the privacy notes
                <ArrowUpRight aria-hidden data-icon="inline-end" />
              </Button>
            </div>
          </div>
        </section>

        {/* Eval */}
        <section className="rule py-14">
          <h2 className="text-2xl md:text-3xl">A benchmark that can fail in public</h2>
          <p className="mt-4 max-w-2xl text-muted-foreground">
            The hard question about a tool like this is whether it finds workflows a person
            would actually have written down.{" "}
            <code className="text-foreground">ritual eval</code> answers it with your own skills
            as an answer key: for each one you wrote by hand, it re-runs the miner over only the
            sessions that predate it and checks whether the workflow shows up in the ranking.
          </p>
          <div className="mt-8 max-w-xl">
            <CopyCommand command="ritual eval" />
          </div>
          <p className="mt-4 text-sm text-muted-foreground">
            It prints how much history predated each skill alongside every result, because a
            miss with five sessions of history says nothing about the tool.
          </p>
        </section>

        {/* Agents */}
        <section id="agents" className="rule scroll-mt-20 py-14">
          <h2 className="text-2xl md:text-3xl">Where your agents keep their history</h2>
          <div className="mt-8 overflow-x-auto rounded-xl border border-border">
            <table className="w-full min-w-[34rem] text-left text-sm">
              <thead className="border-b border-border bg-muted/40">
                <tr>
                  <th className="px-4 py-3 font-medium">agent</th>
                  <th className="px-4 py-3 font-medium">location</th>
                  <th className="px-4 py-3 font-medium">format</th>
                </tr>
              </thead>
              <tbody className="ui">
                {agents.map(([name, path, format]) => (
                  <tr key={name} className="border-b border-border last:border-0">
                    <td className="px-4 py-2.5 whitespace-nowrap">{name}</td>
                    <td className="px-4 py-2.5 text-muted-foreground">{path}</td>
                    <td className="px-4 py-2.5 text-muted-foreground">{format}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="mt-4 text-sm text-muted-foreground">
            Storage layouts move. One file has to change when they do, and a pull request adding
            an agent is about forty lines.
          </p>
        </section>

        {/* Install */}
        <section id="install" className="rule scroll-mt-20 py-14">
          <h2 className="text-2xl md:text-3xl">Install</h2>
          <div className="mt-8 grid gap-10 md:grid-cols-2">
            <InstallTabs />
            <div>
              <h3 className="ui text-sm tracking-[0.14em] text-muted-foreground uppercase">
                Then
              </h3>
              <dl className="mt-4 space-y-3">
                {commands.map(([command, description]) => (
                  <div key={command} className="flex flex-col gap-0.5">
                    <dt className="ui text-sm">{command}</dt>
                    <dd className="text-sm text-muted-foreground">{description}</dd>
                  </div>
                ))}
              </dl>
            </div>
          </div>
        </section>
      </main>

      <footer className="rule">
        <div className="mx-auto flex w-full max-w-5xl flex-col gap-2 px-6 py-10 text-sm text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
          <p>
            Apache 2.0 ·{" "}
            <Link href={REPO} className="transition-colors hover:text-foreground">
              github.com/HarjjotSinghh/ritual
            </Link>
          </p>
          <p>Everything ritual does runs on your machine.</p>
        </div>
      </footer>
    </>
  );
}
