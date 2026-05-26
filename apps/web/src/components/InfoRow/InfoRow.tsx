import styles from "./InfoRow.module.css";

export function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border bg-slate-50 p-2.5">
      <div className="text-xs font-bold text-slate-500">{label}</div>
      <div className="mt-1 break-all font-semibold text-slate-900">{value}</div>
    </div>
  );
}
