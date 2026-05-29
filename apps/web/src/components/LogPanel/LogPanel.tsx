import { Database } from "lucide-react";
import type { Job, RuntimeLog } from "../../types";
import { Card } from "../Card/Card";
import { EmptyState } from "../EmptyState/EmptyState";
import styles from "./LogPanel.module.css";

export function LogPanel({
  logs,
  activeJob,
  fillHeight = false,
}: {
  logs: RuntimeLog[];
  activeJob: Job | null;
  fillHeight?: boolean;
}) {
  return (
    <Card
      title={activeJob ? `任务 #${activeJob.id} 实时日志` : "实时日志"}
      icon={<Database size={18} />}
      className="min-h-0"
    >
      <div
        className={`overflow-y-auto rounded-xl border bg-slate-50 p-2 font-mono text-xs text-slate-700 shadow-inner ${
          fillHeight ? "h-[360px] min-h-0 lg:h-full" : "h-[360px]"
        }`}
      >
        {logs.length === 0 && (
          <EmptyState
            title="暂无日志"
            description="任务运行时，实时日志会显示在这里。"
            compact
          />
        )}
        {logs.map((log) => (
          <div
            key={log.id}
            className="min-w-0 border-b border-slate-200/80 px-2 py-1.5 last:border-b-0"
          >
            <div className="flex min-w-0 items-center gap-2 leading-4">
              <span className="shrink-0 tabular-nums text-slate-400">
                {new Date(log.created_at).toLocaleTimeString()}
              </span>
              <b className="truncate text-blue-600">{log.email}</b>
            </div>
            <div
              className={`mt-0.5 min-w-0 break-words pl-0 leading-5 text-slate-800 ${styles.message}`}
            >
              {log.message}
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}
