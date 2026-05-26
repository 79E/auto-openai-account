import type { ReactNode } from "react";
import styles from "./Card.module.css";

export function Card({
  title,
  icon,
  actions,
  className = "",
  children,
}: {
  title: string;
  icon?: ReactNode;
  actions?: ReactNode;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div
      className={`flex flex-col rounded-2xl border border-slate-200/70 bg-white/80 p-4 shadow-soft backdrop-blur ${className}`}
    >
      <div className="mb-3 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-base font-extrabold">
          {icon}
          {title}
        </div>
        {actions}
      </div>
      <div className="min-h-0 flex-1">{children}</div>
    </div>
  );
}
