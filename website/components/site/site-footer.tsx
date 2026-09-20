import {
  BookText,
  ExternalLink,
  FileCode2,
  GitPullRequest,
  History,
  Map,
  Package,
  Scale,
  ScrollText,
  ShieldCheck,
  SquareTerminal,
  Waypoints,
} from "lucide-react";

import { Shell } from "@/components/layout/shell";
import { BrandMark } from "@/components/site/brand-mark";
import { REPO } from "@/lib/content";

const columns = [
  {
    title: "Source",
    links: [
      { label: "Repository", href: REPO, Icon: FileCode2 },
      { label: "Releases", href: `${REPO}/releases/latest`, Icon: Package },
      {
        label: "Changelog",
        href: `${REPO}/blob/main/CHANGELOG.md`,
        Icon: History,
      },
      { label: "Roadmap", href: `${REPO}/blob/main/ROADMAP.md`, Icon: Map },
    ],
  },
  {
    title: "Docs",
    links: [
      {
        label: "Commands",
        href: `${REPO}/blob/main/docs/commands.md`,
        Icon: SquareTerminal,
      },
      {
        label: "Architecture",
        href: `${REPO}/blob/main/docs/architecture.md`,
        Icon: Waypoints,
      },
      {
        label: "Scoring",
        href: `${REPO}/blob/main/docs/scoring.md`,
        Icon: Scale,
      },
      {
        label: "Privacy",
        href: `${REPO}/blob/main/docs/privacy.md`,
        Icon: ShieldCheck,
      },
    ],
  },
  {
    title: "Project",
    links: [
      {
        label: "Contributing",
        href: `${REPO}/blob/main/CONTRIBUTING.md`,
        Icon: GitPullRequest,
      },
      {
        label: "Security",
        href: `${REPO}/blob/main/SECURITY.md`,
        Icon: ShieldCheck,
      },
      { label: "Licence", href: `${REPO}/blob/main/LICENSE`, Icon: ScrollText },
      { label: "Issues", href: `${REPO}/issues`, Icon: BookText },
    ],
  },
];

export function SiteFooter() {
  return (
    <footer className="mt-auto">
      <Shell className="crop rule-t">
        <div className="grid gap-10 py-14 sm:grid-cols-2 lg:grid-cols-[1.4fr_repeat(3,1fr)]">
          <div className="reveal-item max-w-xs">
            <p className="text-[15px] font-medium tracking-tight">ritual</p>
            <p className="mt-3 text-[13px] text-pretty text-muted-foreground">
              Cross-agent process mining. Reads the session history already on
              your disk and turns the work you repeat into artifacts your agents
              can load.
            </p>
            <a
              href={REPO}
              className="group mt-5 inline-flex items-center gap-2 text-[13px] text-muted-foreground transition-colors duration-(--dur-fast) hover:text-foreground"
            >
              <BrandMark name="github" className="size-3.5" />
              HarjjotSinghh/ritual
              <ExternalLink
                aria-hidden
                className="size-3 transition-transform duration-(--dur-slow) ease-(--ease-smooth-out) group-hover:-translate-y-px group-hover:translate-x-px"
              />
            </a>
          </div>

          {columns.map((column, i) => (
            <div
              key={column.title}
              style={{ "--i": i + 1 } as React.CSSProperties}
              className="reveal-item"
            >
              <p className="label text-muted-foreground">{column.title}</p>
              <ul className="mt-4 space-y-2.5">
                {column.links.map((link) => (
                  <li key={link.label}>
                    <a
                      href={link.href}
                      className="group inline-flex items-center gap-2 text-[13px] text-muted-foreground transition-colors duration-(--dur-fast) hover:text-foreground"
                    >
                      <link.Icon
                        aria-hidden
                        strokeWidth={1.5}
                        className="size-3.5 shrink-0 transition-colors duration-(--dur-fast) group-hover:text-primary"
                      />
                      {link.label}
                    </a>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div className="reveal flex flex-col gap-2 border-t border-border py-6 text-[12.5px] text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
          <p>Apache 2.0. Built by Harjot Singh Rana.</p>
          <p>Everything ritual does runs on your machine.</p>
        </div>
      </Shell>

      {/* The wordmark as a baseline rule. It sits under the column, clipped by
          it, so the page ends on the same two vertical hairlines it started on.
          The mask takes it from nothing at the top to about a tenth at the
          crop, so it reads as the page fading out rather than as a watermark
          someone forgot to delete. */}
      <Shell className="overflow-hidden">
        <p
          aria-hidden
          className="-mb-[0.18em] translate-y-[0.06em] text-[18vw] leading-[0.76] font-medium tracking-[-0.06em] text-foreground/10 select-none [mask-image:linear-gradient(to_bottom,transparent_0%,rgba(0,0,0,0.35)_55%,#000_100%)]"
        >
          ritual
        </p>
      </Shell>
    </footer>
  );
}
