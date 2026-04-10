import { cn } from "@/lib/utils";

export function Badge({
  className,
  tone = "default",
  ...props
}: React.HTMLAttributes<HTMLSpanElement> & {
  tone?: "default" | "success" | "muted";
}) {
  const toneClass =
    tone === "success"
      ? "bg-primary text-primary-foreground"
      : tone === "muted"
        ? "border border-border bg-secondary text-secondary-foreground"
        : "border border-border bg-background text-foreground";

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold",
        toneClass,
        className,
      )}
      {...props}
    />
  );
}
