import {
  BookMarked,
  Download,
  Network,
  ShieldOff,
  Workflow,
} from "lucide-react";

import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { Shell } from "@/components/layout/shell";
import { ThemeToggle } from "@/components/site/theme-toggle";
import { BrandMark } from "@/components/site/brand-mark";
import { REPO } from "@/lib/content";

const links = [
  { href: "#how", label: "How it works", Icon: BookMarked },
  { href: "#verdicts", label: "Verdicts", Icon: Workflow },
  { href: "#privacy", label: "Privacy", Icon: ShieldOff },
  { href: "#agents", label: "Agents", Icon: Network },
];

export function SiteNav() {
  return (
    <header className="sticky top-0 z-20">
      {/* The rules are on the Shell, not on the header, so the bar terminates
          on the column edges. The fixed hairlines behind it are washed out by
          the backdrop blur, which is why it draws its own. */}
      <Shell className="border-x border-b border-border bg-background/85 backdrop-blur-sm">
        <nav className="flex h-14 items-center gap-7">
          <a
            href="#top"
            className="text-[15px] font-medium tracking-tight transition-colors duration-(--dur-fast) hover:text-primary"
          >
            ritual
          </a>

          <div className="hidden items-center gap-5 text-[13px] text-muted-foreground md:flex">
            {links.map((link) => (
              <a
                key={link.href}
                href={link.href}
                className="link-underline group inline-flex items-center gap-1.5 transition-colors duration-(--dur-fast) hover:text-foreground"
              >
                <link.Icon
                  aria-hidden
                  strokeWidth={1.5}
                  className="size-3.5 shrink-0 transition-colors duration-(--dur-fast) group-hover:text-primary"
                />
                {link.label}
              </a>
            ))}
          </div>

          <div className="ml-auto flex items-center gap-1">
            {/* Links are anchors styled with the button recipe, not button
                primitives with an anchor inside: a nav item that navigates
                should keep link semantics. */}
            <a
              href={REPO}
              aria-label="ritual on GitHub"
              className={cn(
                buttonVariants({ variant: "ghost", size: "icon-sm" }),
                "text-muted-foreground hover:text-foreground",
              )}
            >
              <BrandMark name="github" />
            </a>
            <ThemeToggle />
            <a
              href="#install"
              className={cn(buttonVariants({ size: "sm" }), "group ml-2")}
            >
              <Download
                aria-hidden
                data-icon="inline-start"
                className="transition-transform duration-(--dur-slow) ease-(--ease-smooth-out) group-hover:translate-y-px"
              />
              Install
            </a>
          </div>
        </nav>
      </Shell>

      {/* Reading position, taken straight from the scroll timeline. It is the
          only always-on motion besides the caret, and it reports real state. */}
      <Shell className="pointer-events-none px-0">
        <div aria-hidden className="scroll-progress h-px bg-primary" />
      </Shell>
    </header>
  );
}
