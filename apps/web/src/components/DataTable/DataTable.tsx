import type { ReactNode } from "react";
import styles from "./DataTable.module.css";

export function DataTable({
  headers,
  children,
}: {
  headers: string[];
  children: ReactNode;
}) {
  return (
    <div className="overflow-auto rounded-xl border">
      <table className="w-full border-collapse bg-white text-sm">
        <thead>
          <tr>
            {headers.map((h) => (
              <th
                key={h}
                className="border-b bg-slate-50 px-3 py-2 text-left font-bold text-slate-500"
              >
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="[&_td]:border-b [&_td]:px-3 [&_td]:py-2">
          {children}
        </tbody>
      </table>
    </div>
  );
}
