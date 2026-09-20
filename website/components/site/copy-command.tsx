"use client";

import { useEffect, useState } from "react";
import { Check, Copy } from "lucide-react";

import { cn } from "@/lib/utils";

/**
 * A command you are meant to run, with the one affordance it needs. The icon
 * swap is the only state change and it is instant: copying is a
 * high-frequency, keyboard-adjacent action, so it gets feedback, not an
 * animation.
 */
export function CopyCommand({
  command,
  className,
  size = "default",
}: {
  command: string;
  className?: string;
  size?: "default" | "sm";
}) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 1600);
    return () => clearTimeout(timer);
  }, [copied]);

  async function copy() {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
    } catch {
      // Clipboard access can be refused. The command is visible either way,
      // so this fails quietly rather than throwing a dialog at someone.
    }
  }

  return (
    <div
      className={cn(
        "group flex min-w-0 items-center gap-3 rounded-lg border border-border bg-card transition-colors duration-(--dur-slow) hover:border-primary/40",
        size === "sm" ? "px-3 py-2" : "px-4 py-3",
        className,
      )}
    >
      <span aria-hidden className="mono text-sm text-muted-foreground">
        $
      </span>
      <code
        className={cn(
          "min-w-0 flex-1 overflow-x-auto whitespace-nowrap",
          size === "sm" ? "text-[12.5px]" : "text-[13px]",
        )}
      >
        {command}
      </code>
      <button
        type="button"
        onClick={copy}
        aria-label={copied ? "Copied" : `Copy: ${command}`}
        className="-m-2 rounded-md p-2 text-muted-foreground transition-colors duration-(--dur-fast) hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
      >
        {/* The check is keyed so React remounts it, which replays the pop.
            Copying is a keyboard-adjacent action, so the feedback is 150ms and
            the icon never moves the layout around it. */}
        {copied ? (
          <Check
            key="copied"
            className="size-4 animate-in zoom-in-50 duration-(--dur) text-success"
            aria-hidden
          />
        ) : (
          <Copy className="size-4" aria-hidden />
        )}
      </button>
    </div>
  );
}
