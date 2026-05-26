import { Activity } from "lucide-react";
import { jobStatusText, jobTypeText, resultText } from "../../lib/format";
import type { Job, Mailbox } from "../../types";
import { Badge } from "../Badge/Badge";
import { Card } from "../Card/Card";
import { EmptyState } from "../EmptyState/EmptyState";
import { MiniStat } from "../MiniStat/MiniStat";
import styles from "./JobSummary.module.css";

export function JobSummary({
  activeJob,
  mailboxes,
  progress,
  openMailboxDetail,
}: {
  activeJob: Job | null;
  mailboxes: Mailbox[];
  progress: number;
  openMailboxDetail: (mailbox: Mailbox) => void;
}) {
  const activeMailboxes = activeJob
    ? mailboxes.filter((m) => m.last_job_id === activeJob.id)
    : [];
  const jobItems = activeJob?.items || [];
  const taskItems = activeMailboxes.length
    ? activeMailboxes.map((mailbox) => ({
        id: mailbox.id,
        email: mailbox.email,
        mailbox,
        status: mailbox.status,
        text: mailbox.status === "new" ? "未使用" : mailbox.status_text,
        detail: mailbox.current_step_index
          ? `${mailbox.current_step_index}/${mailbox.current_step_total} ${mailbox.current_step}`
          : mailbox.current_step || jobStatusText(mailbox.last_job_status),
      }))
    : jobItems.map((item) => ({
        id: item.id,
        email: item.email,
        mailbox: mailboxes.find((mailbox) => mailbox.email === item.email),
        status: item.status,
        text: resultText(item.status),
        detail: item.error || `${item.duration_ms}ms`,
      }));

  return (
    <Card
      title={`当前任务 ${activeJob ? `#${activeJob.id}` : ""}`}
      icon={<Activity size={18} />}
    >
      <div className="flex h-[360px] min-h-0 flex-col lg:h-full">
        <p className="text-sm text-slate-500">
          {activeJob
            ? `${jobTypeText(activeJob.type)}任务 · ${activeJob.requested_count || activeJob.total_count} 个邮箱 · ${activeJob.success_count + activeJob.failed_count} / ${activeJob.total_count} 完成`
            : "暂无任务。"}
        </p>
        <div className="mt-3 h-2 overflow-hidden rounded-full bg-slate-200">
          <div
            className="h-full bg-gradient-to-r from-blue-600 to-emerald-500"
            style={{ width: `${progress}%` }}
          />
        </div>
        <div className="mt-3 grid grid-cols-3 gap-2">
          <MiniStat label="成功" value={activeJob?.success_count || 0} />
          <MiniStat label="失败" value={activeJob?.failed_count || 0} />
          <MiniStat label="进度" value={progress} />
        </div>
        <div className="mt-4 min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
          {!activeJob && (
            <EmptyState
              title="暂无当前任务"
              description="创建任务后，邮箱执行进度会显示在这里。"
            />
          )}
          {activeJob && taskItems.length === 0 && (
            <EmptyState title="暂无邮箱明细" description="本任务还没有邮箱执行记录。" />
          )}
          {taskItems.map((item) => {
            const canOpen = Boolean(item.mailbox);
            return (
            <button
              type="button"
              key={`${item.email}-${item.id}`}
              onClick={() => item.mailbox && openMailboxDetail(item.mailbox)}
              disabled={!canOpen}
              className={`flex w-full items-center justify-between gap-3 rounded-xl border bg-slate-50 p-3 text-left transition ${
                canOpen
                  ? "hover:border-blue-200 hover:bg-blue-50/50"
                  : "cursor-default"
              }`}
            >
              <div className="min-w-0">
                <div className="truncate font-bold">{item.email}</div>
                <div className="truncate text-sm text-slate-500">
                  {item.detail || "等待任务"}
                </div>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <Badge status={item.status} text={item.text || "-"} />
                {canOpen && (
                  <span className="rounded-full bg-slate-200/70 px-2 py-0.5 text-xs font-bold text-slate-600">
                    详情
                  </span>
                )}
              </div>
            </button>
            );
          })}
        </div>
      </div>
    </Card>
  );
}
