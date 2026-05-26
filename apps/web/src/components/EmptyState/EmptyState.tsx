import styles from "./EmptyState.module.css";

export function EmptyState({
  title,
  description,
  compact = false,
}: {
  title: string;
  description?: string;
  compact?: boolean;
}) {
  return (
    <div
      className={`rounded-2xl border border-dashed border-slate-300 bg-slate-50 px-4 text-center ${
        compact ? "py-5" : "py-10"
      }`}
    >
      <div className="text-sm font-bold text-slate-700">{title}</div>
      {description && (
        <div className="mt-1 text-sm text-slate-500">{description}</div>
      )}
    </div>
  );
}
