export function PageHeader({
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow: string;
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex items-start justify-between border-b border-eidolon-border px-8 py-6">
      <div>
        <div className="font-mono text-[11px] tracking-widest text-eidolon-accent">
          {eyebrow}
        </div>
        <h1 className="mt-1 text-xl font-semibold text-eidolon-text">{title}</h1>
        {description && (
          <p className="mt-1 text-sm text-eidolon-muted">{description}</p>
        )}
      </div>
      {action}
    </div>
  );
}
