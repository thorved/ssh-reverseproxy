import { cn } from "@/lib/utils";

export function Select({
  className,
  ...props
}: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      className={cn(
        "flex h-11 w-full rounded-2xl border border-input bg-background/55 px-4 py-2 text-sm shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] outline-none backdrop-blur transition focus-visible:ring-2 focus-visible:ring-ring dark:bg-white/5",
        className,
      )}
      {...props}
    />
  );
}
