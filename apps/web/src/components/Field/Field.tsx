import type { ReactNode } from "react";
import styles from "./Field.module.css";

export function Field({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <label className="mt-3 block">
      <span className="mb-1.5 block text-sm font-bold text-slate-600">
        {label}
      </span>
      {children}
    </label>
  );
}
