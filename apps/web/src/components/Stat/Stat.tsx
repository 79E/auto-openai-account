import styles from "./Stat.module.css";

export function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-xl border bg-white p-3">
      <div className="text-xs font-bold text-slate-500">{label}</div>
      <div className="mt-0.5 text-xl font-black tracking-tight">{value}</div>
    </div>
  );
}
