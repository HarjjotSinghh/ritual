import { cn } from "@/lib/utils";
import { Shell } from "@/components/layout/shell";

/**
 * A page section. The rule and the crop marks live on the Shell rather than on
 * the section element, so a section divider ends exactly where the column's
 * vertical hairlines are instead of running out into the hatched margin.
 */
export function Section({
  id,
  label,
  title,
  lede,
  children,
  className,
  headerClassName,
}: {
  id?: string;
  label?: string;
  title?: React.ReactNode;
  lede?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
  headerClassName?: string;
}) {
  return (
    <section id={id} className="scroll-mt-14">
      <Shell className={cn("crop rule-t py-16 md:py-24", className)}>
        {(label || title || lede) && (
          <header className={cn("reveal max-w-2xl", headerClassName)}>
            {label && (
              <p className="label mb-4 text-muted-foreground">{label}</p>
            )}
            {title && (
              <h2 className="text-2xl leading-[1.15] text-balance md:text-[2rem]">
                {title}
              </h2>
            )}
            {lede && (
              <p className="mt-4 text-pretty text-muted-foreground md:text-base">
                {lede}
              </p>
            )}
          </header>
        )}
        {children}
      </Shell>
    </section>
  );
}
