import { cn } from "@/lib/utils";
import { brandMarks, type BrandKey } from "@/lib/brand-marks";

/**
 * A brand mark, inlined from lib/brand-marks.ts. The paths are static author
 * data extracted at build time, so dangerouslySetInnerHTML here carries no
 * untrusted input; it is how you inline an SVG body without shipping the
 * whole icon package.
 */
export function BrandMark({
  name,
  className,
  title,
}: {
  name: BrandKey;
  className?: string;
  title?: string;
}) {
  const mark = brandMarks[name];

  return (
    <svg
      viewBox={mark.viewBox}
      role={title ? "img" : undefined}
      aria-label={title}
      aria-hidden={title ? undefined : true}
      fill="currentColor"
      fillRule={mark.fillRule ? "evenodd" : undefined}
      clipRule={mark.fillRule ? "evenodd" : undefined}
      className={cn("size-4 shrink-0", className)}
      dangerouslySetInnerHTML={{ __html: mark.body }}
    />
  );
}
