export function StatCard({
  label,
  value,
  tone = "default",
}: {
  label: string;
  value: string;
  tone?: "default" | "danger" | "ok";
}) {
  const toneClass =
    tone === "danger"
      ? "text-eidolon-danger"
      : tone === "ok"
        ? "text-eidolon-ok"
        : "text-eidolon-text";

  return (
    <div className="rounded-lg border border-eidolon-border bg-eidolon-surface p-4">
      <div className="font-mono text-[11px] uppercase tracking-wider text-eidolon-muted">
        {label}
      </div>
      <div className={`mt-2 font-mono text-2xl font-medium ${toneClass}`}>
        {value}
      </div>
    </div>
  );
}
