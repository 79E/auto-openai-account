import styles from "./Badge.module.css";

export function Badge({ status, text }: { status: string; text: string }) {
  const cls = ["registered", "success", "finished"].includes(status)
    ? "bg-emerald-100 text-emerald-700"
    : ["abnormal", "failed"].includes(status)
      ? "bg-rose-100 text-rose-700"
      : ["registering", "running"].includes(status)
        ? "bg-amber-100 text-amber-700"
        : "bg-blue-100 text-blue-700";
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-bold ${cls}`}>
      {text}
    </span>
  );
}
