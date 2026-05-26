import { useState } from "react";
import { Database, UploadCloud } from "lucide-react";
import { jobTypeText, resultText } from "../../lib/format";
import type { Mailbox, MailboxView } from "../../types";
import { Badge } from "../../components/Badge/Badge";
import { Card } from "../../components/Card/Card";
import { DataTable } from "../../components/DataTable/DataTable";
import { EmptyState } from "../../components/EmptyState/EmptyState";
import { MiniStat } from "../../components/MiniStat/MiniStat";
import { Modal } from "../../components/Modal/Modal";
import styles from "./MailboxesPage.module.css";

export function MailboxesPage({
  mailboxes,
  importText,
  setImportText,
  importMailboxes,
  openMailboxDetail,
  deleteMailboxes,
  resetMailboxes,
  startLoginJob,
  busy,
}: {
  mailboxes: Mailbox[];
  importText: string;
  setImportText: (value: string) => void;
  importMailboxes: () => void;
  openMailboxDetail: (mailbox: Mailbox) => void;
  deleteMailboxes: (ids: number[]) => void;
  resetMailboxes: (ids: number[]) => void;
  startLoginJob: (ids: number[]) => void;
  busy: boolean;
}) {
  const [selected, setSelected] = useState<number[]>([]);
  const [importOpen, setImportOpen] = useState(false);
  const [view, setView] = useState<MailboxView>("all");
  const [page, setPage] = useState(1);
  const counts = mailboxes.reduce<Record<string, number>>((a, m) => {
    a[m.status] = (a[m.status] || 0) + 1;
    return a;
  }, {});
  const usedCount = mailboxes.length - (counts.new || 0);
  const tabs: { key: MailboxView; label: string; value: number }[] = [
    { key: "all", label: "全部", value: mailboxes.length },
    { key: "unused", label: "未使用", value: counts.new || 0 },
    { key: "used", label: "已使用", value: usedCount },
    { key: "registered", label: "已注册", value: counts.registered || 0 },
    { key: "abnormal", label: "异常", value: counts.abnormal || 0 },
  ];
  const visible = mailboxes.filter((m) => {
    if (view === "all") return true;
    if (view === "unused") return m.status === "new";
    if (view === "used") return m.status !== "new";
    return m.status === view;
  });
  const pageSize = 50;
  const totalPages = Math.max(1, Math.ceil(visible.length / pageSize));
  const currentPage = Math.min(page, totalPages);
  const pageItems = visible.slice(
    (currentPage - 1) * pageSize,
    currentPage * pageSize,
  );
  const allSelected =
    pageItems.length > 0 && pageItems.every((m) => selected.includes(m.id));
  const toggleOne = (id: number) =>
    setSelected((p) =>
      p.includes(id) ? p.filter((x) => x !== id) : [...p, id],
    );
  const toggleAll = () => {
    const pageIds = pageItems.map((m) => m.id);
    setSelected((prev) =>
      allSelected
        ? prev.filter((id) => !pageIds.includes(id))
        : Array.from(new Set([...prev, ...pageIds])),
    );
  };
  async function submitImport() {
    await importMailboxes();
    setImportOpen(false);
  }
  async function confirmDelete() {
    if (
      selected.length &&
      window.confirm(`确定删除 ${selected.length} 个邮箱吗？`)
    ) {
      await deleteMailboxes(selected);
      setSelected([]);
    }
  }
  async function confirmReset() {
    if (
      selected.length &&
      window.confirm(`确定重置 ${selected.length} 个邮箱为未使用吗？`)
    ) {
      await resetMailboxes(selected);
      setSelected([]);
    }
  }
  return (
    <div className="space-y-4">
      <Card title="邮箱池" icon={<Database size={18} />}>
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="grid grid-cols-2 gap-2 md:grid-cols-5">
            <MiniStat label="全部" value={mailboxes.length} />
            <MiniStat label="未使用" value={counts.new || 0} />
            <MiniStat label="已使用" value={usedCount} />
            <MiniStat label="已注册" value={counts.registered || 0} />
            <MiniStat label="异常" value={counts.abnormal || 0} />
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              onClick={() => setImportOpen(true)}
              className="inline-flex items-center gap-2 rounded-xl bg-slate-950 px-3 py-2 font-bold text-white"
            >
              <UploadCloud size={16} />
              批量导入
            </button>
            <button
              onClick={() => startLoginJob(selected)}
              disabled={busy || selected.length === 0}
              className="rounded-xl border bg-white px-3 py-2 font-bold disabled:opacity-50"
            >
              批量登录
            </button>
            <button
              onClick={confirmReset}
              disabled={busy || selected.length === 0}
              className="rounded-xl border bg-white px-3 py-2 font-bold disabled:opacity-50"
            >
              重置未使用
            </button>
            <button
              onClick={confirmDelete}
              disabled={busy || selected.length === 0}
              className="rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 font-bold text-rose-700 disabled:opacity-50"
            >
              批量删除
            </button>
          </div>
        </div>
      </Card>
      <Card title="邮箱列表" icon={<Database size={18} />}>
        <div className="mb-3 flex flex-wrap gap-2">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => {
                setView(tab.key);
                setPage(1);
                setSelected([]);
              }}
              className={
                view === tab.key
                  ? "rounded-xl bg-slate-950 px-3 py-1.5 font-bold text-white"
                  : "rounded-xl border bg-white px-3 py-1.5 font-bold"
              }
            >
              {tab.label}
              <span className="ml-2 opacity-70">{tab.value}</span>
            </button>
          ))}
        </div>
        <DataTable headers={["", "邮箱", "状态", "任务", "结果", "操作"]} minWidth="52rem">
          {pageItems.map((m) => (
            <tr key={m.id}>
              <td>
                <input
                  type="checkbox"
                  checked={selected.includes(m.id)}
                  onChange={() => toggleOne(m.id)}
                />
              </td>
              <td className="font-semibold">{m.email}</td>
              <td>
                <Badge
                  status={m.status}
                  text={m.status === "new" ? "未使用" : m.status_text}
                />
              </td>
              <td>
                {m.last_job_id
                  ? `#${m.last_job_id} ${jobTypeText(m.last_job_type)}`
                  : "-"}
              </td>
              <td>
                {m.last_job_status ? (
                  <Badge
                    status={m.last_job_status}
                    text={resultText(m.last_job_status)}
                  />
                ) : (
                  "-"
                )}
                {m.last_job_error && (
                  <div className="mt-1 max-w-56 truncate text-xs text-rose-600">
                    {m.last_job_error}
                  </div>
                )}
              </td>
              <td>
                <div className="flex gap-2">
                  <button
                    onClick={() => openMailboxDetail(m)}
                    className="rounded-xl border bg-white px-3 py-2 text-xs font-bold"
                  >
                    详情
                  </button>
                  <button
                    onClick={() => startLoginJob([m.id])}
                    className="rounded-xl border bg-white px-3 py-2 text-xs font-bold"
                  >
                    登录
                  </button>
                  <button
                    onClick={() => resetMailboxes([m.id])}
                    className="rounded-xl border bg-white px-3 py-2 text-xs font-bold"
                  >
                    重置
                  </button>
                </div>
              </td>
            </tr>
          ))}
        </DataTable>
        {visible.length === 0 && (
          <div className="mt-3">
            <EmptyState
              title="暂无邮箱"
              description={
                mailboxes.length === 0
                  ? "点击上方“批量导入”添加邮箱后，可在这里查看和管理。"
                  : "当前筛选条件下没有邮箱，切换分类后再查看。"
              }
            />
          </div>
        )}
        <div className="mt-3 flex items-center gap-3 text-sm text-slate-500">
          <label className="inline-flex items-center gap-2">
            <input type="checkbox" checked={allSelected} onChange={toggleAll} />
            全选当前页
          </label>
          <span>
            已选择 {selected.length} 个邮箱，当前显示 {pageItems.length} / {visible.length} 个
          </span>
        </div>
        {visible.length > pageSize && (
          <div className="mt-3 flex flex-col gap-2 rounded-xl border bg-slate-50 p-3 text-sm text-slate-600 sm:flex-row sm:items-center sm:justify-between">
            <span>
              第 {currentPage} / {totalPages} 页，每页 {pageSize} 条
            </span>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => setPage(1)}
                disabled={currentPage === 1}
                className="rounded-xl border bg-white px-3 py-2 font-bold disabled:opacity-50"
              >
                首页
              </button>
              <button
                type="button"
                onClick={() => setPage(currentPage - 1)}
                disabled={currentPage === 1}
                className="rounded-xl border bg-white px-3 py-2 font-bold disabled:opacity-50"
              >
                上一页
              </button>
              <button
                type="button"
                onClick={() => setPage(currentPage + 1)}
                disabled={currentPage === totalPages}
                className="rounded-xl border bg-white px-3 py-2 font-bold disabled:opacity-50"
              >
                下一页
              </button>
              <button
                type="button"
                onClick={() => setPage(totalPages)}
                disabled={currentPage === totalPages}
                className="rounded-xl border bg-white px-3 py-2 font-bold disabled:opacity-50"
              >
                末页
              </button>
            </div>
          </div>
        )}
      </Card>
      {importOpen && (
        <Modal title="批量导入邮箱" onClose={() => setImportOpen(false)}>
          <textarea
            value={importText}
            onChange={(e) => setImportText(e.target.value)}
            placeholder="email@example.com----password----client_id----refresh_token"
            className="h-60 w-full rounded-xl border bg-white p-3 font-mono text-sm outline-none focus:ring-2 focus:ring-blue-500"
          />
          <div className="mt-4 flex justify-end gap-2">
            <button
              onClick={() => setImportOpen(false)}
              className="rounded-xl border bg-white px-3 py-2 font-bold"
            >
              取消
            </button>
            <button
              onClick={submitImport}
              disabled={busy}
              className="rounded-xl bg-slate-950 px-3 py-2 font-bold text-white disabled:opacity-50"
            >
              确认导入
            </button>
          </div>
        </Modal>
      )}
    </div>
  );
}
