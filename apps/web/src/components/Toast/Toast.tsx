import type { ToastState } from "../../types";
import styles from "./Toast.module.css";

export function Toast({ toast, onClose }: { toast: ToastState; onClose: () => void }) {
  if (!toast) return null;
  const cls =
    toast.type === "success"
      ? "border-emerald-200 bg-emerald-50 text-emerald-800"
      : toast.type === "error"
        ? "border-rose-200 bg-rose-50 text-rose-800"
        : "border-blue-200 bg-blue-50 text-blue-800";
  return (
    <div className="fixed right-4 top-4 z-[80] w-[min(380px,calc(100vw-2rem))]">
      <div
        className={`rounded-xl border px-3 py-2.5 shadow-soft backdrop-blur ${cls}`}
      >
        <div className="flex items-start justify-between gap-3">
          <div className="text-sm font-bold leading-6">{toast.message}</div>
          <button
            onClick={onClose}
            className="rounded-full px-2 text-lg leading-6 opacity-60 transition hover:opacity-100"
          >
            ×
          </button>
        </div>
      </div>
    </div>
  );
}
