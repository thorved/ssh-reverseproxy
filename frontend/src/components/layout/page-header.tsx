import { Badge } from "@/components/ui/badge";

export function PageHeader({
  eyebrow,
  title,
  description,
  badge,
  actions,
}: {
  eyebrow?: string;
  title: string;
  description: string;
  badge?: string;
  actions?: React.ReactNode;
}) {
  return (
    <header className="flex flex-col gap-4 pb-4 sm:flex-row sm:items-start sm:justify-between">
      <div className="space-y-1.5">
        {eyebrow ? (
          <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
            {eyebrow}
          </p>
        ) : null}
        <div className="flex items-center gap-3">
          <h1 className="text-3xl font-semibold tracking-tight text-foreground">
            {title}
          </h1>
          {badge ? <Badge>{badge}</Badge> : null}
        </div>
        <p className="text-base text-muted-foreground">{description}</p>
      </div>
      {actions ? <div className="shrink-0">{actions}</div> : null}
    </header>
  );
}
