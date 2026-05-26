import type { ReactNode } from "react";
import styles from "./Modal.module.css";

export function Modal({
  title,
  subtitle,
  onClose,
  children,
}: {
  title: string;
  subtitle?: string;
  onClose: () => void;
  children: ReactNode;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/40 p-3 backdrop-blur-sm">
      <div className="w-full max-w-2xl rounded-2xl border bg-white p-4 shadow-soft">
        <div className="mb-3 flex items-center justify-between">
          <div className="min-w-0">
            <h2 className="text-lg font-black">{title}</h2>
            {subtitle && (
              <p className="mt-1 break-all font-mono text-sm text-slate-500">
                {subtitle}
              </p>
            )}
          </div>
          <button
            onClick={onClose}
            className="rounded-full border px-3 py-1 text-slate-500"
          >
            关闭
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}
