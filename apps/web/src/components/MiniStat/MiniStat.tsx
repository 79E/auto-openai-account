import styles from "./MiniStat.module.css";

export function MiniStat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-xl border bg-slate-50 px-3 py-2">
      <div className="text-xs font-bold text-slate-500">{label}</div>
      <div className="mt-0.5 text-lg font-black">{value}</div>
    </div>
  );
}
