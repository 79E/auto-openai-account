import type { ReactNode } from "react";
import styles from "./Modal.module.css";

export function Modal({
  title,
  subtitle,
  onClose,
  children,
  footer,
}: {
  title: string;
  subtitle?: string;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/40 p-3 backdrop-blur-sm">
      <div className="flex max-h-[calc(100vh-3rem)] w-full max-w-2xl flex-col overflow-hidden rounded-2xl border bg-white p-4 shadow-soft">
        <div className="mb-3 flex shrink-0 items-center justify-between">
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
              className="self-start pt-0.5 text-slate-400 hover:text-slate-600"
            >
              ✕
            </button>
        </div>
        <div className="overflow-y-auto">{children}</div>
        {footer && <div className="shrink-0 pt-3">{footer}</div>}
      </div>
    </div>
  );
}
