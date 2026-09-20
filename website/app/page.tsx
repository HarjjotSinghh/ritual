import { ArrowUpRight, Lock, ShieldOff, Terminal } from "lucide-react";

import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Shell } from "@/components/layout/shell";
import { Section } from "@/components/layout/section";
import { SiteNav } from "@/components/site/site-nav";
import { SiteFooter } from "@/components/site/site-footer";
import { BrandMark } from "@/components/site/brand-mark";
import { Console } from "@/components/site/console";
import { CopyCommand } from "@/components/site/copy-command";
import { InstallTabs } from "@/components/site/install-tabs";
import { Faq } from "@/components/site/faq";
import {
  INSTALL_COMMAND,
  REPO,
  agents,
  claims,
  commands,
  evidenceOutput,
  scanOutput,
  verdicts,
} from "@/lib/content";

const noCodeFor = ["An account", "An upload", "Telemetry", "A hosted version"];

export default function Home() {
  return (
    <>
      <SiteNav />

      <main id="top" className="flex-1">
        {/* Hero. Asymmetric split: the argument on the left, the list of things
            it reads on the right, because the list is the product claim. */}
        <Shell>
          <div className="grid gap-12 pt-16 pb-14 md:pt-20 lg:grid-cols-12 lg:gap-10 lg:pt-24">
            <div className="min-w-0 lg:col-span-7">
              <p className="label text-muted-foreground">
                Cross-agent process mining
              </p>
              <h1
                className="mt-5 max-w-[16ch] text-[2.125rem] leading-[1.04] text-balance sm:text-[2.5rem] md:text-[3.25rem] md:leading-[1.02] lg:text-[3.5rem]">
                Your agents already wrote down how you work.
              </h1>
              <p
                className="mt-6 max-w-[46ch] text-base text-pretty text-muted-foreground md:text-[1.0625rem]"
              >
                Months of agent transcripts are sitting on your disk right now.
                Nothing reads them. ritual does.
              </p>

              <div className="mt-9 max-w-lg">
                <CopyCommand command={INSTALL_COMMAND} />
              </div>
              <div className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-2">
                <a
                  href="#install"
                  className="link-underline group inline-flex items-center gap-1.5 text-[13px] text-muted-foreground transition-colors duration-(--dur-fast) hover:text-foreground"
                >
                  <Terminal
                    aria-hidden
                    className="size-3.5 transition-colors duration-(--dur-fast) group-hover:text-primary"
                  />
                  Other ways to install
                </a>
                <a
                  href={REPO}
                  className="link-underline group inline-flex items-center gap-1.5 text-[13px] text-muted-foreground transition-colors duration-(--dur-fast) hover:text-foreground"
                >
                  <BrandMark name="github" className="size-3.5" />
                  Read the source
                  <ArrowUpRight
                    aria-hidden
                    className="size-3.5 transition-transform duration-(--dur-slow) ease-(--ease-smooth-out) group-hover:-translate-y-px group-hover:translate-x-px"
                  />
                </a>
              </div>
            </div>

            <aside className="min-w-0 lg:col-span-5 lg:pl-10">
              <div className="flex items-baseline justify-between border-b border-border pb-2.5">
                <p className="label text-muted-foreground">Reads history from</p>
                <p className="mono tabular text-xs text-muted-foreground">
                  {agents.length}
                </p>
              </div>
              <ul className="mono grid grid-cols-2 gap-x-6 text-[13px] sm:grid-cols-3 lg:grid-cols-2">
                {agents.map((agent) => (
                  <li
                    key={agent.name}
                    className="group flex items-center gap-2.5 border-b border-border py-2 whitespace-nowrap"
                  >
                    <BrandMark
                      name={agent.icon}
                      className="size-3.5 text-muted-foreground transition-colors duration-(--dur-fast) group-hover:text-foreground"
                    />
                    {agent.name}
                  </li>
                ))}
              </ul>
              <p className="mt-4 text-[13px] text-pretty text-muted-foreground">
                One normalized shape. A workflow you run in Cursor on Monday and
                in Codex on Thursday is one workflow.
              </p>
            </aside>
          </div>
        </Shell>

        {/* The output, full width in the column. This is the product, so it is
            set as a code block rather than dressed as a screenshot. */}
        <Section className="py-10 md:py-12">
          <Console
            className="reveal"
            command="ritual scan"
            lines={scanOutput}
            caption="Example output. Yours will name your own work."
          />
        </Section>

        {/* Three claims as a numbered run against a ruled column, not as three
            matching cards. They are not peers: scope, then the hard part, then
            the reason to believe either. */}
        <Section
          id="how"
          title="Not another skill generator"
          lede="Several tools turn Claude Code history into skills. ritual differs in three ways that matter."
        >
          <div className="mt-12 grid gap-px border-t border-border md:mt-16 md:grid-cols-3 md:border-t-0">
            {claims.map((claim, i) => (
              <article
                key={claim.n}
                style={{ "--i": i } as React.CSSProperties}
                className="reveal-item group border-b border-border py-6 md:border-b-0 md:border-t md:py-0 md:pt-6 md:pr-8 md:last:pr-0"
              >
                <div className="flex items-center gap-2.5">
                  <claim.Icon
                    aria-hidden
                    strokeWidth={1.5}
                    className="size-[18px] text-primary transition-transform duration-(--dur-slow) ease-(--ease-smooth-out) group-hover:-translate-y-0.5"
                  />
                  <p className="mono tabular text-xs text-muted-foreground">
                    {claim.n}
                  </p>
                </div>
                <h3 className="mt-3.5 text-[15px] font-medium text-balance">
                  {claim.title}
                </h3>
                <p className="mt-2.5 max-w-[42ch] text-sm text-pretty text-muted-foreground">
                  {claim.body}
                </p>
              </article>
            ))}
          </div>
        </Section>

        {/* The six verdicts that write something. Six cells, three across, so
            the grid closes evenly at every breakpoint. */}
        <Section
          id="verdicts"
          label="The classifier"
          title="A repeated behaviour can be six different things"
          lede="Putting it in the wrong one is worse than leaving it alone. A standing preference written as a skill never fires, because nothing invokes it."
        >
          <div className="mt-12 grid gap-px bg-border md:grid-cols-2 lg:grid-cols-3">
            {verdicts.map((row, i) => (
              <article
                key={row.kind}
                style={{ "--i": i } as React.CSSProperties}
                className="reveal-item group bg-background p-5"
              >
                <div className="flex items-center gap-2.5">
                  <row.Icon
                    aria-hidden
                    strokeWidth={1.5}
                    className="size-[18px] text-primary transition-transform duration-(--dur-slow) ease-(--ease-smooth-out) group-hover:-translate-y-0.5"
                  />
                  <p className="label text-primary">{row.kind}</p>
                </div>
                <p className="mt-3.5 text-sm text-pretty">{row.when}</p>
                <p className="mt-1.5 text-sm text-pretty text-muted-foreground">
                  {row.why}
                </p>
              </article>
            ))}
          </div>
          <p className="reveal mt-5 flex items-start gap-2 text-[13px] text-pretty text-muted-foreground">
            <ShieldOff
              aria-hidden
              strokeWidth={1.5}
              className="mt-0.5 size-4 shrink-0"
            />
            There is a seventh verdict. <code>ignore</code> is what a real
            pattern gets when it is not worth an artifact, and it writes
            nothing: git status runs a hundred times a week.
          </p>
        </Section>

        {/* Evidence: the prose is short because the output is the argument. */}
        <Section title="No finding is a vibe">
          <p className="reveal mt-4 max-w-2xl text-pretty text-muted-foreground">
            Every suggestion carries the transcripts it came from, the days it
            happened on, and what share of runs each step appeared in. Open one
            and check it.
          </p>
          <Console
            className="reveal mt-10"
            command="ritual show 360119359c"
            lines={evidenceOutput}
          />
        </Section>

        {/* Privacy. A statement, then the specific denials, then the thing most
            tools in this category will not say out loud. */}
        <Section id="privacy" label="Local-first">
          <div className="grid gap-12 lg:grid-cols-12 lg:gap-10">
            <div className="reveal-item min-w-0 lg:col-span-7">
              <h2 className="max-w-[18ch] text-2xl leading-[1.15] text-balance md:text-[2rem]">
                Nothing leaves your machine.
              </h2>
              <div className="mt-6 max-w-[54ch] space-y-4 text-pretty text-muted-foreground">
                <p>
                  Not as a setting you can turn off, but because the input is
                  your entire working history. It contains client code, customer
                  data, internal URLs, and every credential anyone ever pasted
                  into a prompt.
                </p>
                <p>
                  So ritual is a single binary that reads local files and writes
                  local files. Secrets are stripped at the read boundary, home
                  paths become{" "}
                  <code className="text-foreground">${"{HOME}"}</code>, and the
                  dashboard binds to loopback behind a token.
                </p>
              </div>
              <a
                href={`${REPO}/blob/main/docs/privacy.md`}
                className={cn(
                  buttonVariants({ variant: "outline", size: "sm" }),
                  "group mt-7",
                )}
              >
                <Lock aria-hidden data-icon="inline-start" />
                Read the privacy notes
                <ArrowUpRight
                  aria-hidden
                  data-icon="inline-end"
                  className="transition-transform duration-(--dur-slow) ease-(--ease-smooth-out) group-hover:-translate-y-px group-hover:translate-x-px"
                />
              </a>
            </div>

            <div
              style={{ "--i": 1 } as React.CSSProperties}
              className="reveal-item min-w-0 lg:col-span-5 lg:pl-10"
            >
              <p className="label text-muted-foreground">
                What there is no code for
              </p>
              <ul className="mt-4 border-t border-border">
                {noCodeFor.map((item) => (
                  <li
                    key={item}
                    className="group flex items-center gap-2.5 border-b border-border py-2.5 text-sm"
                  >
                    <ShieldOff
                      aria-hidden
                      strokeWidth={1.5}
                      className="size-4 shrink-0 text-muted-foreground transition-colors duration-(--dur-fast) group-hover:text-foreground"
                    />
                    {item}
                  </li>
                ))}
              </ul>
              <p className="mt-5 text-sm text-pretty text-muted-foreground">
                The obvious product is a website: run a command, upload the
                output, get skills back. It will not be built. Asking a
                developer to upload six months of transcripts is asking them to
                upload their employer&rsquo;s source code.
              </p>
            </div>
          </div>
        </Section>

        {/* The benchmark. A single claim and the command that tests it. */}
        <Section>
          <div className="reveal max-w-3xl">
            <h2 className="text-2xl leading-[1.15] text-balance md:text-[2rem]">
              A benchmark that can fail in public
            </h2>
            <p className="mt-4 text-pretty text-muted-foreground">
              The hard question about a tool like this is whether it finds
              workflows a person would actually have written down.{" "}
              <code className="text-foreground">ritual eval</code> answers it
              with your own skills as an answer key: for each one you wrote by
              hand, it re-runs the miner over only the sessions that predate it
              and checks whether the workflow shows up in the ranking.
            </p>
            <div className="reveal mt-8 max-w-md">
              <CopyCommand command="ritual eval" size="sm" />
            </div>
            <p className="reveal mt-4 text-[13px] text-pretty text-muted-foreground">
              It prints how much history predated each skill alongside every
              result, because a miss with five sessions of history says nothing
              about the tool.
            </p>
          </div>
        </Section>

        {/* Reference data. A table, because the content genuinely is tabular. */}
        <Section
          id="agents"
          title="Where your agents keep their history"
          lede="Storage layouts move. One file has to change when they do, and a pull request adding an agent is about forty lines."
        >
          <div className="reveal mt-10 overflow-x-auto">
            <Table className="min-w-[38rem]">
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="label h-auto pb-2.5 pl-0 text-muted-foreground">
                    Agent
                  </TableHead>
                  <TableHead className="label h-auto pb-2.5 text-muted-foreground">
                    Location
                  </TableHead>
                  <TableHead className="label h-auto pr-0 pb-2.5 text-right text-muted-foreground">
                    Format
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className="mono">
                {agents.map((agent) => (
                  <TableRow
                    key={agent.name}
                    className="group transition-colors duration-(--dur-fast) hover:bg-muted/40"
                  >
                    <TableCell className="py-2.5 pl-0 whitespace-nowrap">
                      <span className="flex items-center gap-2.5">
                        <BrandMark
                          name={agent.icon}
                          className="size-4 text-muted-foreground transition-colors duration-(--dur-fast) group-hover:text-foreground"
                        />
                        {agent.name}
                      </span>
                    </TableCell>
                    <TableCell className="py-2.5 text-muted-foreground">
                      {agent.path}
                    </TableCell>
                    <TableCell className="py-2.5 pr-0 text-right text-muted-foreground">
                      {agent.format}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </Section>

        {/* Install. Tabs for the four routes in, the command surface on the
            right so the first thing after installing is on the same screen. */}
        <Section id="install" label="Install" title="Four ways in">
          <div className="mt-10 grid gap-12 lg:grid-cols-12 lg:gap-10">
            <div className="reveal-item min-w-0 lg:col-span-6">
              <InstallTabs />
            </div>

            <div
              style={{ "--i": 1 } as React.CSSProperties}
              className="reveal-item min-w-0 lg:col-span-6 lg:pl-10"
            >
              <p className="label text-muted-foreground">Then</p>
              <dl className="mt-4 border-t border-border">
                {commands.map((entry) => (
                  <div
                    key={entry.command}
                    className="group flex flex-col gap-0.5 border-b border-border py-3 sm:flex-row sm:items-baseline sm:gap-6"
                  >
                    <dt className="mono flex w-48 shrink-0 items-center gap-2.5 text-[13px]">
                      <entry.Icon
                        aria-hidden
                        strokeWidth={1.5}
                        className="size-4 shrink-0 text-muted-foreground transition-colors duration-(--dur-fast) group-hover:text-primary"
                      />
                      {entry.command}
                    </dt>
                    <dd className="text-[13px] text-pretty text-muted-foreground">
                      {entry.description}
                    </dd>
                  </div>
                ))}
              </dl>
            </div>
          </div>
        </Section>

        <Section title="Questions worth asking before you run it">
          <div className="reveal mt-10">
            <Faq />
          </div>
        </Section>
      </main>

      <SiteFooter />
    </>
  );
}
