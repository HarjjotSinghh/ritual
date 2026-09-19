import { cn } from "@/lib/utils";

type Line = {
  text: string;
  tone?: "dim" | "accent" | "good" | "warn" | "bold";
  indent?: boolean;
};

const tones: Record<NonNullable<Line["tone"]>, string> = {
  dim: "text-muted-foreground",
  accent: "text-primary",
  good: "text-success",
  warn: "text-warning",
  bold: "text-foreground font-semibold",
};

export function Terminal({
  title,
  command,
  lines,
  className,
}: {
  title: string;
  command: string;
  lines: Line[];
  className?: string;
}) {
  return (
    <figure
      className={cn(
        "overflow-hidden rounded-xl border border-border bg-card",
        className,
      )}
    >
      <figcaption className="flex items-center gap-2 border-b border-border px-4 py-2.5">
        <span aria-hidden className="flex gap-1.5">
          <span className="size-2.5 rounded-full bg-border" />
          <span className="size-2.5 rounded-full bg-border" />
          <span className="size-2.5 rounded-full bg-border" />
        </span>
        <span className="ui ml-1 text-xs text-muted-foreground">{title}</span>
      </figcaption>

      <div className="overflow-x-auto px-4 py-4">
        <pre className="tabular text-[12.5px] leading-[1.75] md:text-[13px]">
          <code>
            <span className="text-muted-foreground">$ </span>
            <span className="font-semibold">{command}</span>
            {"\n"}
            {lines.map((line, i) => (
              <span key={i} className={cn(tones[line.tone ?? "dim"], !line.tone && "text-foreground")}>
                {line.text}
                {"\n"}
              </span>
            ))}
          </code>
        </pre>
      </div>
    </figure>
  );
}
