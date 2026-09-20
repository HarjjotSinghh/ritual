import { SquareTerminal } from "lucide-react";

import { cn } from "@/lib/utils";
import type { ConsoleLine } from "@/lib/content";

const tones: Record<NonNullable<ConsoleLine["tone"]>, string> = {
  dim: "text-muted-foreground",
  accent: "text-primary",
  good: "text-success",
  warn: "text-warning",
  bold: "text-foreground font-medium",
};

/**
 * Real command output, set as a code block rather than dressed up as a fake
 * macOS window. The product is a terminal tool, so the output is the
 * screenshot; traffic-light dots would only add furniture around it.
 */
export function Console({
  command,
  lines,
  caption,
  className,
}: {
  command: string;
  lines: ConsoleLine[];
  caption?: string;
  className?: string;
}) {
  return (
    <figure className={cn("min-w-0", className)}>
      <div className="overflow-hidden rounded-lg border border-border bg-card">
        <div className="flex items-center gap-2 border-b border-border px-4 py-2">
          <SquareTerminal
            aria-hidden
            strokeWidth={1.5}
            className="size-3.5 shrink-0 text-muted-foreground"
          />
          <span aria-hidden className="mono text-xs text-muted-foreground">
            $
          </span>
          <span className="mono truncate text-xs font-medium">{command}</span>
          <span aria-hidden className="caret ml-0.5 shrink-0 text-primary" />
        </div>

        <div className="overflow-x-auto px-4 py-4">
          <pre className="tabular text-[12px] leading-[1.7] md:text-[12.5px]">
            <code>
              {lines.map((line, i) => (
                <span
                  key={i}
                  className={cn(
                    tones[line.tone ?? "dim"],
                    !line.tone && "text-foreground",
                  )}
                >
                  {line.text}
                  {"\n"}
                </span>
              ))}
            </code>
          </pre>
        </div>
      </div>

      {caption && (
        <figcaption className="mt-3 text-sm text-pretty text-muted-foreground">
          {caption}
        </figcaption>
      )}
    </figure>
  );
}
