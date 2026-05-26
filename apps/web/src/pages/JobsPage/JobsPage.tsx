import { Activity } from "lucide-react";
import { jobStatusText, jobTypeText } from "../../lib/format";
import type { Job, Mailbox, RuntimeLog } from "../../types";
import { Badge } from "../../components/Badge/Badge";
import { Card } from "../../components/Card/Card";
import { EmptyState } from "../../components/EmptyState/EmptyState";
import { JobDetail } from "../../components/JobDetail/JobDetail";
import { LogPanel } from "../../components/LogPanel/LogPanel";
import { MiniStat } from "../../components/MiniStat/MiniStat";
import styles from "./JobsPage.module.css";

export function JobsPage({
  jobs,
  activeJob,
  logs,
  mailboxes,
  openTask,
  openMailboxDetail,
  stopTask,
  exportJobTokens,
  selectJob,
  busy,
}: {
  jobs: Job[];
  activeJob: Job | null;
  logs: RuntimeLog[];
  mailboxes: Mailbox[];
  openTask: () => void;
  openMailboxDetail: (mailbox: Mailbox) => void;
  stopTask: (id: number) => void;
  exportJobTokens: (job: Job) => void;
  selectJob: (id: number) => void;
  busy: boolean;
}) {
  return (
    <div className="grid min-h-0 gap-4 lg:h-[calc(100vh-5.75rem)] lg:grid-cols-[.85fr_1.15fr]">
      <div className="flex min-h-0 flex-col rounded-2xl border border-slate-200/70 bg-white/80 p-4 shadow-soft backdrop-blur">
        <div className="mb-3 flex items-center justify-between gap-3">
          <div className="flex items-center gap-2 text-base font-extrabold">
            <Activity size={18} />
            任务列表
          </div>
          <button
            onClick={openTask}
            disabled={busy}
            className="rounded-xl bg-slate-950 px-3 py-2 font-bold text-white disabled:opacity-50"
          >
            创建任务
          </button>
        </div>
        <div className="min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
          {jobs.length === 0 && (
            <EmptyState
              title="暂无任务"
              description="点击右上角“创建任务”开始注册或登录换 token。"
            />
          )}
          {jobs.map((j) => (
            <div
              key={j.id}
              onClick={() => selectJob(j.id)}
              role="button"
              tabIndex={0}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  selectJob(j.id);
                }
              }}
              className={`w-full rounded-xl border p-3 text-left transition hover:border-blue-200 hover:bg-blue-50/40 ${
                activeJob?.id === j.id
                  ? "border-blue-300 bg-blue-50 shadow-sm"
                  : "bg-slate-50"
              }`}
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-base font-black">#{j.id}</div>
                  <div className="mt-1 text-sm text-slate-500">
                    {jobTypeText(j.type)}任务 · {j.total_count} 个邮箱
                  </div>
                </div>
                <Badge status={j.status} text={jobStatusText(j.status)} />
              </div>
              <div className="mt-3 grid grid-cols-4 gap-2 text-center text-sm">
                <MiniStat label="成功" value={j.success_count} />
                <MiniStat label="失败" value={j.failed_count} />
                <MiniStat label="总数" value={j.total_count} />
                <MiniStat label="成功率" value={Number(j.success_rate.toFixed(1))} />
              </div>
              <div className="mt-3 flex items-center justify-between gap-2">
                <div className="h-2 flex-1 overflow-hidden rounded-full bg-slate-200">
                  <div
                    className="h-full bg-gradient-to-r from-blue-600 to-emerald-500"
                    style={{
                      width: `${Math.max(0, Math.min(100, j.success_rate))}%`,
                    }}
                  />
                </div>
                {j.status === "running" ? (
                  <button
                    type="button"
                    onClick={(event) => {
                      event.stopPropagation();
                      stopTask(j.id);
                    }}
                    disabled={busy}
                    className="rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-xs font-bold text-rose-700 disabled:opacity-50"
                  >
                    结束
                  </button>
                ) : null}
              </div>
            </div>
          ))}
        </div>
      </div>
      <div className="grid min-h-0 gap-4 lg:grid-rows-[minmax(0,1fr)_minmax(0,1fr)]">
        <JobDetail
          job={activeJob}
          mailboxes={mailboxes}
          openMailboxDetail={openMailboxDetail}
          exportJobTokens={exportJobTokens}
          busy={busy}
        />
        <LogPanel logs={logs} activeJob={activeJob} fillHeight />
      </div>
    </div>
  );
}
