import { cn } from "@/lib/utils";

export function Textarea({
  className,
  ...props
}: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return (
    <textarea
      className={cn(
        "flex min-h-32 w-full rounded-2xl border border-input bg-background/55 px-4 py-3 text-sm shadow-[inset_0_1px_0_rgba(255,255,255,0.08)] outline-none backdrop-blur transition placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring dark:bg-white/5",
        className,
      )}
      {...props}
    />
  );
}
