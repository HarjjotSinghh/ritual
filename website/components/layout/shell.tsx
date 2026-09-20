import { cn } from "@/lib/utils";

/**
 * The ruled column. Everything on the page sits inside one of these so that
 * section rules, the nav and the footer all terminate on the same two vertical
 * hairlines drawn by `.shell-rules` in the root layout.
 */
export function Shell({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return <div className={cn("shell", className)} {...props} />;
}

/**
 * A horizontal rule across the column, with crop marks where it meets the
 * vertical rules. Used as the separator between page sections.
 */
export function Rule({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      aria-hidden
      className={cn("crop rule-t h-0", className)}
      {...props}
    />
  );
}
