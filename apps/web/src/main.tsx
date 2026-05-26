import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  Activity,
  Database,
  Play,
  PlugZap,
  RefreshCw,
  UploadCloud,
} from "lucide-react";
import "./styles.css";

type Page = "overview" | "mailboxes" | "jobs" | "proxies" | "plugins";
type MailboxView = "all" | "unused" | "used" | "registered" | "abnormal";
type Stats = {
  mailboxes: Record<string, number>;
  jobs: Record<string, number>;
};
type Mailbox = {
  id: number;
  email: string;
  password?: string;
  client_id?: string;
  access_token?: string;
  register_password?: string;
  token_json?: string;
  status: string;
  status_text: string;
  current_step?: string;
  current_step_index?: number;
  current_step_total?: number;
  proxy?: string;
  last_error?: string;
  last_job_id?: number;
  last_job_type?: string;
  last_job_status?: string;
  last_job_error?: string;
};
type MailboxUpdate = Pick<
  Mailbox,
  "email" | "password" | "client_id" | "access_token" | "register_password"
>;
type Job = {
  id: number;
  type: string;
  status: string;
  requested_count: number;
  total_count: number;
  success_count: number;
  failed_count: number;
  success_rate: number;
  items?: JobItem[];
};
type JobItem = {
  id: number;
  email: string;
  status: string;
  error?: string;
  duration_ms: number;
};
type RuntimeLog = {
  id: number;
  email: string;
  level: string;
  step: string;
  step_index: number;
  step_total: number;
  message: string;
  created_at: string;
};
type SettingsPayload = {
  proxy_mode: string;
  proxies: string[];
  register_concurrency: number;
  password_mode: string;
  fixed_password: string;
  imap_host: string;
  imap_port: number;
  imap_auth_mode: string;
  otp_timeout_seconds: number;
  otp_poll_interval_seconds: number;
  listen: string;
};
type ProxyTestResult = {
  proxy: string;
  ok: boolean;
  ip?: string;
  latency_ms: number;
  error?: string;
};
type ToastState = {
  message: string;
  type: "success" | "error" | "info";
} | null;

const emptyStats: Stats = { mailboxes: {}, jobs: {} };
const defaultPassword = "Mima1234567890.";
const nav: Array<{ key: Page; label: string }> = [
  { key: "overview", label: "总览" },
  { key: "mailboxes", label: "邮箱池" },
  { key: "jobs", label: "任务" },
  { key: "proxies", label: "代理池" },
  { key: "plugins", label: "插件" },
];

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok)
    throw new Error(
      (await res.json().catch(() => ({}))).error || res.statusText,
    );
  return res.json();
}

function normalizeSettingsPayload(settings: SettingsPayload): SettingsPayload {
  return {
    ...settings,
    proxies: Array.isArray(settings.proxies) ? settings.proxies : [],
    fixed_password: settings.fixed_password || defaultPassword,
  };
}

function App() {
  const [page, setPage] = useState<Page>("overview");
  const [stats, setStats] = useState<Stats>(emptyStats);
  const [mailboxes, setMailboxes] = useState<Mailbox[]>([]);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [activeJob, setActiveJob] = useState<Job | null>(null);
  const [latestJob, setLatestJob] = useState<Job | null>(null);
  const [logs, setLogs] = useState<RuntimeLog[]>([]);
  const [latestLogs, setLatestLogs] = useState<RuntimeLog[]>([]);
  const [settings, setSettings] = useState<SettingsPayload | null>(null);
  const [settingsDraft, setSettingsDraft] = useState<SettingsPayload | null>(
    null,
  );
  const [importText, setImportText] = useState("");
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState<ToastState>(null);
  const [taskOpen, setTaskOpen] = useState(false);
  const [mailboxDetail, setMailboxDetail] = useState<Mailbox | null>(null);
  const [mailboxDetailDraft, setMailboxDetailDraft] =
    useState<MailboxUpdate | null>(null);
  const [credentialsOpen, setCredentialsOpen] = useState(false);

  function showToast(
    message: string,
    type: "success" | "error" | "info" = "info",
  ) {
    setToast({ message, type });
  }

  useEffect(() => {
    if (!toast) return;
    const timer = window.setTimeout(() => setToast(null), 3200);
    return () => window.clearTimeout(timer);
  }, [toast]);

  async function fetchJobSnapshot(id: number) {
    const detail = await api<Job>(`/api/register-jobs/${id}`);
    const logData = await api<{ items: RuntimeLog[] }>(
      `/api/register-jobs/${id}/logs`,
    );
    return { detail, logs: logData.items || [] };
  }

  async function loadJob(id: number) {
    const snapshot = await fetchJobSnapshot(id);
    setActiveJob(snapshot.detail);
    setLogs(snapshot.logs);
  }

  async function loadLatestJob(id: number) {
    const snapshot = await fetchJobSnapshot(id);
    setLatestJob(snapshot.detail);
    setLatestLogs(snapshot.logs);
    return snapshot;
  }

  async function refresh(preferredJobId?: number) {
    const [statsData, mailboxData, jobData, settingsData] = await Promise.all([
      api<Stats>("/api/stats"),
      api<{ total: number; items: Mailbox[] }>("/api/mailboxes?page_size=200"),
      api<{ total: number; items: Job[] }>("/api/register-jobs?page_size=20"),
      api<SettingsPayload>("/api/settings"),
    ]);
    setStats(statsData);
    setMailboxes(mailboxData.items || []);
    setJobs(jobData.items || []);
    const normalizedSettings = normalizeSettingsPayload(settingsData);
    setSettings(normalizedSettings);
    setSettingsDraft(normalizedSettings);
    const latest = jobData.items?.[0] || null;
    const latestSnapshot = latest ? await loadLatestJob(latest.id) : null;
    const targetId = preferredJobId || activeJob?.id || latest?.id;
    const selected = targetId
      ? jobData.items?.find((job) => job.id === targetId)
      : null;
    if (selected) {
      if (latestSnapshot && selected.id === latestSnapshot.detail.id) {
        setActiveJob(latestSnapshot.detail);
        setLogs(latestSnapshot.logs);
      } else {
        await loadJob(selected.id);
      }
    } else {
      setActiveJob(null);
      setLogs([]);
    }
    if (!latest) {
      setLatestJob(null);
      setLatestLogs([]);
    }
  }

  useEffect(() => {
    refresh().catch(console.error);
  }, []);

  useEffect(() => {
    if (!activeJob?.id || activeJob.status !== "running") return;
    if (activeJob.id === latestJob?.id) return;
    const source = new EventSource(`/api/register-jobs/${activeJob.id}/events`);
    source.addEventListener("log", (event) => {
      const entry = JSON.parse((event as MessageEvent).data) as RuntimeLog;
      setLogs((prev) => [...prev.slice(-80), entry]);
      refresh(activeJob.id).catch(console.error);
    });
    return () => source.close();
  }, [activeJob?.id, activeJob?.status, latestJob?.id]);

  useEffect(() => {
    if (!latestJob?.id || latestJob.status !== "running") return;
    const source = new EventSource(`/api/register-jobs/${latestJob.id}/events`);
    source.addEventListener("log", (event) => {
      const entry = JSON.parse((event as MessageEvent).data) as RuntimeLog;
      setLatestLogs((prev) => [...prev.slice(-80), entry]);
      if (activeJob?.id === latestJob.id) {
        setLogs((prev) => [...prev.slice(-80), entry]);
      }
      refresh(activeJob?.id).catch(console.error);
    });
    return () => source.close();
  }, [latestJob?.id, latestJob?.status, activeJob?.id]);

  async function saveSettings(next: SettingsPayload) {
    const saved = await api<{ settings: SettingsPayload }>("/api/settings", {
      method: "PUT",
      body: JSON.stringify(normalizeSettingsPayload(next)),
    });
    const normalizedSettings = normalizeSettingsPayload(saved.settings);
    setSettings(normalizedSettings);
    setSettingsDraft(normalizedSettings);
    return normalizedSettings;
  }

  async function importMailboxes() {
    setBusy(true);
    try {
      const result = await api<{
        imported: number;
        skipped: number;
        failed: number;
      }>("/api/mailboxes/import", {
        method: "POST",
        body: JSON.stringify({ text: importText }),
      });
      showToast(
        `导入完成：新增 ${result.imported}，跳过 ${result.skipped}，失败 ${result.failed}`,
        "success",
      );
      await refresh();
    } catch (error) {
      showToast(error instanceof Error ? error.message : "导入失败", "error");
    } finally {
      setBusy(false);
    }
  }

  async function createRegisterTask(config: SettingsPayload, count: number) {
    setBusy(true);
    try {
      await saveSettings(config);
      const job = await api<Job>("/api/register-jobs", {
        method: "POST",
        body: JSON.stringify({ count }),
      });
      setActiveJob(job);
      setPage("jobs");
      setTaskOpen(false);
      showToast(`注册任务 #${job.id} 已启动`, "success");
      await refresh(job.id);
    } catch (error) {
      showToast(
        error instanceof Error ? error.message : "启动注册任务失败",
        "error",
      );
    } finally {
      setBusy(false);
    }
  }

  async function createLoginTask(config: SettingsPayload, ids: number[]) {
    if (ids.length === 0) return;
    setBusy(true);
    try {
      await saveSettings(config);
      const job = await api<Job>("/api/login-jobs", {
        method: "POST",
        body: JSON.stringify({ mailbox_ids: ids }),
      });
      setActiveJob(job);
      setPage("jobs");
      setTaskOpen(false);
      showToast(`登录任务 #${job.id} 已启动`, "success");
      await refresh(job.id);
    } catch (error) {
      showToast(
        error instanceof Error ? error.message : "启动登录任务失败",
        "error",
      );
    } finally {
      setBusy(false);
    }
  }

  async function deleteMailboxes(ids: number[]) {
    if (ids.length === 0) return;
    setBusy(true);
    try {
      await Promise.all(
        ids.map((id) => api(`/api/mailboxes/${id}`, { method: "DELETE" })),
      );
      showToast(`已删除 ${ids.length} 个邮箱`, "success");
      await refresh();
    } catch (error) {
      showToast(
        error instanceof Error ? error.message : "删除邮箱失败",
        "error",
      );
    } finally {
      setBusy(false);
    }
  }

  async function resetMailboxes(ids: number[]) {
    if (ids.length === 0) return;
    setBusy(true);
    try {
      await Promise.all(
        ids.map((id) =>
          api(`/api/mailboxes/${id}`, {
            method: "PUT",
            body: JSON.stringify({ status: "new" }),
          }),
        ),
      );
      showToast(`已重置 ${ids.length} 个邮箱为未使用`, "success");
      await refresh();
    } catch (error) {
      showToast(
        error instanceof Error ? error.message : "重置邮箱失败",
        "error",
      );
    } finally {
      setBusy(false);
    }
  }

  async function updateMailbox(id: number, updates: MailboxUpdate) {
    setBusy(true);
    try {
      const result = await api<{ item: Mailbox }>(`/api/mailboxes/${id}`, {
        method: "PUT",
        body: JSON.stringify(updates),
      });
      setMailboxes((items) =>
        items.map((item) => (item.id === id ? result.item : item)),
      );
      showToast("邮箱详情已保存", "success");
      await refresh();
      return result.item;
    } catch (error) {
      showToast(error instanceof Error ? error.message : "保存邮箱失败", "error");
      throw error;
    } finally {
      setBusy(false);
    }
  }

  function openMailboxDetail(mailbox: Mailbox) {
    setMailboxDetail(mailbox);
    setMailboxDetailDraft({
      email: mailbox.email || "",
      password: mailbox.password || "",
      client_id: mailbox.client_id || "",
      access_token: mailbox.access_token || "",
      register_password: mailbox.register_password || "",
    });
    setCredentialsOpen(false);
  }

  function closeMailboxDetail() {
    setMailboxDetail(null);
    setMailboxDetailDraft(null);
  }

  function updateMailboxDetailDraft(key: keyof MailboxUpdate, value: string) {
    setMailboxDetailDraft((draft) =>
      draft ? { ...draft, [key]: value } : draft,
    );
  }

  function updateCredentialLine(value: string) {
    const [email = "", password = "", clientId = "", ...tokenParts] =
      value.split("----");
    setMailboxDetailDraft((draft) =>
      draft
        ? {
            ...draft,
            email,
            password,
            client_id: clientId,
            access_token: tokenParts.join("----"),
          }
        : draft,
    );
  }

  async function saveMailboxDetail() {
    if (!mailboxDetail || !mailboxDetailDraft) return;
    const saved = await updateMailbox(mailboxDetail.id, mailboxDetailDraft);
    setMailboxDetail(saved);
    setMailboxDetailDraft({
      email: saved.email || "",
      password: saved.password || "",
      client_id: saved.client_id || "",
      access_token: saved.access_token || "",
      register_password: saved.register_password || "",
    });
  }

  async function stopTask(id: number) {
    if (!window.confirm(`确定结束任务 #${id} 吗？`)) return;
    setBusy(true);
    try {
      const result = await api<{ job: Job }>(`/api/register-jobs/${id}/stop`, {
        method: "POST",
      });
      setActiveJob(result.job);
      showToast(`任务 #${id} 已结束`, "success");
      await refresh();
    } catch (error) {
      showToast(
        error instanceof Error ? error.message : "结束任务失败",
        "error",
      );
    } finally {
      setBusy(false);
    }
  }

  const registered = stats.mailboxes.registered || 0;
  const abnormal = stats.mailboxes.abnormal || 0;
  const newCount = stats.mailboxes.new || 0;
  const runningCount = stats.mailboxes.registering || 0;
  const loginingCount = stats.mailboxes.logining || 0;

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top_left,#dbeafe_0,#f8fafc_34%,#eef2ff_100%)] text-slate-950">
      <div className="mx-auto max-w-[92rem] px-4 py-4 sm:px-5">
        <header className="mb-4 flex items-center justify-between gap-3">
          <button
            onClick={() => setPage("overview")}
            className="flex items-center gap-2 font-extrabold"
          >
            <div className="h-8 w-8 rounded-xl bg-gradient-to-br from-blue-600 to-violet-600 shadow-soft" />
            Auto OpenAI Account
          </button>
          <nav className="hidden rounded-xl border border-slate-200/70 bg-white/70 p-1 shadow-sm backdrop-blur lg:flex">
            {nav.map((item) => (
              <button
                key={item.key}
                onClick={() => setPage(item.key)}
                className={
                  page === item.key
                    ? "rounded-lg bg-white px-3 py-1.5 font-semibold shadow-sm"
                    : "px-3 py-1.5 text-slate-500 transition hover:text-slate-950"
                }
              >
                {item.label}
              </button>
            ))}
          </nav>
        </header>

        <Toast toast={toast} onClose={() => setToast(null)} />
        {taskOpen && settingsDraft && (
          <CreateTaskModal
            settings={settingsDraft}
            mailboxes={mailboxes}
            busy={busy}
            onClose={() => setTaskOpen(false)}
            onCreateRegister={createRegisterTask}
            onCreateLogin={createLoginTask}
          />
        )}
        {mailboxDetail && mailboxDetailDraft && (
          <MailboxDetailModal
            detail={mailboxDetail}
            detailDraft={mailboxDetailDraft}
            credentialsOpen={credentialsOpen}
            busy={busy}
            onClose={closeMailboxDetail}
            onToggleCredentials={() => setCredentialsOpen((open) => !open)}
            onUpdateDraft={updateMailboxDetailDraft}
            onUpdateCredentialLine={updateCredentialLine}
            onSave={saveMailboxDetail}
          />
        )}
        {page === "overview" && (
          <Overview
            stats={{
              newCount,
              runningCount,
              loginingCount,
              registered,
              abnormal,
              proxyCount: settings?.proxies?.length || 0,
            }}
            mailboxes={mailboxes}
            logs={latestLogs}
            activeJob={latestJob}
            busy={busy}
            openTask={() => setTaskOpen(true)}
            openMailboxDetail={openMailboxDetail}
            refresh={refresh}
          />
        )}
        {page === "mailboxes" && (
          <MailboxesPage
            mailboxes={mailboxes}
            importText={importText}
            setImportText={setImportText}
            importMailboxes={importMailboxes}
            openMailboxDetail={openMailboxDetail}
            deleteMailboxes={deleteMailboxes}
            resetMailboxes={resetMailboxes}
            startLoginJob={(ids) =>
              settingsDraft && createLoginTask(settingsDraft, ids)
            }
            busy={busy}
          />
        )}
        {page === "jobs" && (
          <JobsPage
            jobs={jobs}
            activeJob={activeJob}
            logs={logs}
            mailboxes={mailboxes}
            openTask={() => setTaskOpen(true)}
            openMailboxDetail={openMailboxDetail}
            stopTask={stopTask}
            selectJob={loadJob}
            busy={busy}
          />
        )}
        {page === "proxies" && settingsDraft && (
          <ProxyPoolPage
            settingsDraft={settingsDraft}
            setSettingsDraft={setSettingsDraft}
            showToast={showToast}
            saveSettings={(next) =>
              saveSettings(next)
                .then(() => showToast("代理池已更新", "success"))
                .catch((e) =>
                  showToast(
                    e instanceof Error ? e.message : "保存失败",
                    "error",
                  ),
                )
            }
          />
        )}
        {page === "plugins" && <PluginsPage />}
      </div>
    </div>
  );
}

function Overview({
  stats,
  mailboxes,
  logs,
  activeJob,
  busy,
  openTask,
  openMailboxDetail,
  refresh,
}: {
  stats: {
    newCount: number;
    runningCount: number;
    loginingCount: number;
    registered: number;
    abnormal: number;
    proxyCount: number;
  };
  mailboxes: Mailbox[];
  logs: RuntimeLog[];
  activeJob: Job | null;
  busy: boolean;
  openTask: () => void;
  openMailboxDetail: (mailbox: Mailbox) => void;
  refresh: () => void;
}) {
  const progress =
    activeJob && activeJob.total_count
      ? Math.round(
          ((activeJob.success_count + activeJob.failed_count) /
            activeJob.total_count) *
            100,
        )
      : 0;
  return (
    <div className="flex min-h-0 flex-col lg:h-[calc(100vh-5.75rem)]">
      <section className="mb-4 shrink-0 grid gap-4 lg:grid-cols-[1fr_1.25fr]">
        <div className="grid grid-cols-2 gap-2 rounded-2xl border border-slate-200/70 bg-white/80 p-3 shadow-soft backdrop-blur sm:grid-cols-3">
          <Stat label="代理池" value={stats.proxyCount} />
          <Stat label="未使用" value={stats.newCount} />
          <Stat label="注册中" value={stats.runningCount} />
          <Stat label="登陆中" value={stats.loginingCount} />
          <Stat label="已注册" value={stats.registered} />
          <Stat label="异常" value={stats.abnormal} />
        </div>
        <div className="rounded-2xl border border-slate-200/70 bg-white/80 p-4 shadow-soft backdrop-blur">
          <p className="mb-1 text-xs font-bold uppercase tracking-wide text-slate-500">Modern SaaS Console</p>
          <h1 className="max-w-3xl text-2xl font-black tracking-[-0.04em]">
            批量注册、代理池和实时日志在一个清爽控制台里。
          </h1>
          <p className="mt-2 text-sm text-slate-500">
            从创建任务开始，选择注册或登录换 token，并配置并发、密码和代理策略。
          </p>
          <div className="mt-4 flex flex-wrap justify-end gap-2">
            <button
              onClick={openTask}
              disabled={busy}
              className="inline-flex items-center gap-2 rounded-xl bg-slate-950 px-3 py-2 font-bold text-white shadow-lg disabled:opacity-50"
            >
              <Play size={16} />
              创建任务
            </button>
            <button
              onClick={refresh}
              className="inline-flex items-center gap-2 rounded-xl border bg-white px-3 py-2 font-bold"
            >
              <RefreshCw size={16} />
              刷新
            </button>
          </div>
        </div>
      </section>
      <section className="grid min-h-0 flex-1 gap-4 lg:grid-cols-2">
        <JobSummary
          activeJob={activeJob}
          mailboxes={mailboxes}
          progress={progress}
          openMailboxDetail={openMailboxDetail}
        />
        <LogPanel logs={logs} activeJob={activeJob} fillHeight />
      </section>
    </div>
  );
}

function CreateTaskModal({
  settings,
  mailboxes,
  busy,
  onClose,
  onCreateRegister,
  onCreateLogin,
}: {
  settings: SettingsPayload;
  mailboxes: Mailbox[];
  busy: boolean;
  onClose: () => void;
  onCreateRegister: (settings: SettingsPayload, count: number) => void;
  onCreateLogin: (settings: SettingsPayload, ids: number[]) => void;
}) {
  const unused = mailboxes.filter((item) => item.status === "new");
  const used = mailboxes.filter((item) => item.status !== "new");
  const [taskType, setTaskType] = useState<"register" | "login">("register");
  const [draft, setDraft] = useState<SettingsPayload>({
    ...settings,
    fixed_password: settings.fixed_password || defaultPassword,
  });
  const [count, setCount] = useState(Math.min(1, unused.length));
  const [loginFilter, setLoginFilter] = useState("used");
  const loginCandidates = used.filter(
    (item) => loginFilter === "used" || item.status === loginFilter,
  );
  function submit() {
    if (taskType === "register")
      onCreateRegister(draft, Math.max(1, Math.min(count, unused.length)));
    else
      onCreateLogin(
        draft,
        loginCandidates.map((item) => item.id),
      );
  }
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/40 p-3 backdrop-blur-sm">
      <div className="w-full max-w-3xl rounded-2xl border bg-white p-4 shadow-soft">
        <div className="mb-4 flex items-center justify-between">
          <div>
            <h2 className="text-lg font-black">创建任务</h2>
            <p className="mt-1 text-sm text-slate-500">
              选择任务类型，并配置本次任务参数。
            </p>
          </div>
          <button
            onClick={onClose}
            className="rounded-full border px-3 py-1 text-slate-500"
          >
            关闭
          </button>
        </div>
        <div className="mb-3 flex gap-2">
          <button
            onClick={() => setTaskType("register")}
            className={
              taskType === "register"
                ? "rounded-xl bg-slate-950 px-3 py-1.5 font-bold text-white"
                : "rounded-xl border bg-white px-3 py-1.5 font-bold"
            }
          >
            注册任务
          </button>
          <button
            onClick={() => setTaskType("login")}
            className={
              taskType === "login"
                ? "rounded-xl bg-slate-950 px-3 py-1.5 font-bold text-white"
                : "rounded-xl border bg-white px-3 py-1.5 font-bold"
            }
          >
            登录换 token
          </button>
        </div>
        <div className="grid gap-3 md:grid-cols-2">
          <Field label="并发数量">
            <input
              className="input"
              type="number"
              min={1}
              value={draft.register_concurrency}
              onChange={(e) =>
                setDraft({
                  ...draft,
                  register_concurrency: Number(e.target.value),
                })
              }
            />
          </Field>
          <Field label="代理模式">
            <select
              className="input"
              value={draft.proxy_mode}
              onChange={(e) =>
                setDraft({ ...draft, proxy_mode: e.target.value })
              }
            >
              <option value="random">随机</option>
              <option value="single">固定第一条</option>
              <option value="round_robin">轮询</option>
            </select>
          </Field>
        </div>
        {taskType === "register" && (
          <div className="mt-3 grid gap-3 md:grid-cols-2">
            <Field label={`注册数量（可用未使用邮箱 ${unused.length} 个）`}>
              <input
                className="input"
                type="number"
                min={1}
                max={unused.length}
                value={count}
                onChange={(e) => setCount(Number(e.target.value))}
              />
            </Field>
            <Field label="密码模式">
              <select
                className="input"
                value={draft.password_mode}
                onChange={(e) =>
                  setDraft({ ...draft, password_mode: e.target.value })
                }
              >
                <option value="random">随机生成</option>
                <option value="fixed">固定密码</option>
              </select>
            </Field>
            {draft.password_mode === "fixed" && (
              <Field label="固定密码">
                <input
                  className="input"
                  value={draft.fixed_password || defaultPassword}
                  onChange={(e) =>
                    setDraft({ ...draft, fixed_password: e.target.value })
                  }
                />
              </Field>
            )}
          </div>
        )}
        {taskType === "login" && (
          <div className="mt-3 grid gap-3 md:grid-cols-1">
            <Field label="邮箱状态筛选">
              <select
                className="input"
                value={loginFilter}
                onChange={(e) => setLoginFilter(e.target.value)}
              >
                <option value="used">全部已使用</option>
                <option value="registered">已注册</option>
                <option value="abnormal">异常</option>
              </select>
              <p className="mt-2 text-sm text-slate-500">
                <span className="font-bold text-slate-800">
                  {loginCandidates.length}
                </span>{" "}
                个邮箱将创建登录任务
              </p>
            </Field>
          </div>
        )}
        <div className="mt-4 flex justify-end gap-2">
          <button
            onClick={onClose}
            className="rounded-xl border bg-white px-3 py-2 font-bold"
          >
            取消
          </button>
          <button
            onClick={submit}
            disabled={
              busy ||
              (taskType === "register"
                ? unused.length === 0
                : loginCandidates.length === 0)
            }
            className="rounded-xl bg-slate-950 px-3 py-2 font-bold text-white disabled:opacity-50"
          >
            创建任务
          </button>
        </div>
      </div>
    </div>
  );
}

function MailboxesPage({
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
  const allSelected = visible.length > 0 && selected.length === visible.length;
  const toggleOne = (id: number) =>
    setSelected((p) =>
      p.includes(id) ? p.filter((x) => x !== id) : [...p, id],
    );
  const toggleAll = () =>
    setSelected(allSelected ? [] : visible.map((m) => m.id));
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
        <DataTable headers={["", "邮箱", "状态", "任务", "结果", "操作"]}>
          {visible.map((m) => (
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
        <div className="mt-3 flex items-center gap-3 text-sm text-slate-500">
          <label className="inline-flex items-center gap-2">
            <input type="checkbox" checked={allSelected} onChange={toggleAll} />
            全选当前列表
          </label>
          <span>已选择 {selected.length} 个邮箱</span>
        </div>
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

function JobsPage({
  jobs,
  activeJob,
  logs,
  mailboxes,
  openTask,
  openMailboxDetail,
  stopTask,
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
            <div className="rounded-xl border bg-slate-50 p-3 text-sm text-slate-500">
              暂无任务。
            </div>
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
        />
        <LogPanel logs={logs} activeJob={activeJob} fillHeight />
      </div>
    </div>
  );
}

function ProxyPoolPage({
  settingsDraft,
  setSettingsDraft,
  showToast,
  saveSettings,
}: {
  settingsDraft: SettingsPayload;
  setSettingsDraft: (s: SettingsPayload) => void;
  showToast: (message: string, type?: "success" | "error" | "info") => void;
  saveSettings: (next: SettingsPayload) => void;
}) {
  const [results, setResults] = useState<Record<string, ProxyTestResult>>({});
  const [testing, setTesting] = useState("");
  const [addOpen, setAddOpen] = useState(false);
  const [addText, setAddText] = useState("");
  const persist = (proxies: string[]) => {
    const next = { ...settingsDraft, proxies };
    setSettingsDraft(next);
    saveSettings(next);
  };
  const remove = (i: number) =>
    persist(settingsDraft.proxies.filter((_, idx) => idx !== i));
  function addFromText() {
    const next = addText
      .split("\n")
      .map((item) => item.trim())
      .filter(Boolean);
    if (!next.length) return;
    const invalid = next.find((item) => !isValidProxyURL(item));
    if (invalid) {
      showToast(`代理格式不正确：${invalid}`, "error");
      return;
    }
    persist(
      Array.from(new Set([...settingsDraft.proxies.filter(Boolean), ...next])),
    );
    setAddText("");
    setAddOpen(false);
  }
  async function test(proxy: string) {
    if (!proxy.trim()) return;
    setTesting(proxy);
    try {
      const d = await api<{ items: ProxyTestResult[] }>("/api/proxy/test", {
        method: "POST",
        body: JSON.stringify({ proxy }),
      });
      setResults((p) => ({ ...p, [proxy]: d.items[0] }));
    } finally {
      setTesting("");
    }
  }
  async function testAll() {
    const ps = settingsDraft.proxies.filter(Boolean);
    if (!ps.length) return;
    setTesting("__all__");
    try {
      const d = await api<{ items: ProxyTestResult[] }>("/api/proxy/test", {
        method: "POST",
        body: JSON.stringify({ proxies: ps }),
      });
      setResults(Object.fromEntries(d.items.map((x) => [x.proxy, x])));
    } finally {
      setTesting("");
    }
  }
  return (
    <>
      <Card title="代理池" icon={<PlugZap size={18} />}>
        <div className="mb-3 flex justify-end gap-2">
          <button
            onClick={() => setAddOpen(true)}
            className="rounded-xl border bg-white px-3 py-2 text-sm font-bold"
          >
            新增代理
          </button>
          <button
            onClick={testAll}
            disabled={testing === "__all__"}
            className="rounded-xl border bg-white px-3 py-2 text-sm font-bold disabled:opacity-50"
          >
            测试全部
          </button>
        </div>
        <div className="space-y-2">
          {settingsDraft.proxies.map((proxy, i) => {
            const r = results[proxy];
            return (
              <div key={i} className="rounded-xl border bg-slate-50 p-3">
                <div className="flex flex-col gap-2 lg:flex-row lg:items-center">
                  <div className="min-w-0 flex-1 rounded-xl border border-slate-200 bg-white px-3 py-2 font-mono text-sm text-slate-800">
                    <div className="truncate">{proxy}</div>
                  </div>
                  <button
                    onClick={() => test(proxy)}
                    disabled={testing === proxy}
                    className="rounded-xl bg-slate-950 px-3 py-2 text-sm font-bold text-white disabled:opacity-50"
                  >
                    测速
                  </button>
                  <button
                    onClick={() => remove(i)}
                    className="rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm font-bold text-rose-700"
                  >
                    删除
                  </button>
                </div>
                {r && (
                  <div className="mt-2 rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm text-slate-600">
                    <div className="flex flex-wrap items-center gap-2">
                      <Badge
                        status={r.ok ? "success" : "failed"}
                        text={r.ok ? "可用" : "失败"}
                      />
                      <span>IP：{r.ip || "-"}</span>
                      <span>延迟：{r.latency_ms}ms</span>
                      {r.error && (
                        <span className="text-rose-600">{r.error}</span>
                      )}
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </Card>
      {addOpen && (
        <Modal
          title="新增代理"
          subtitle="一行一个代理，支持 http、https、socks5、socks5h"
          onClose={() => setAddOpen(false)}
        >
          <textarea
            value={addText}
            onChange={(e) => setAddText(e.target.value)}
            placeholder="socks5://127.0.0.1:7890\nhttp://127.0.0.1:8080"
            className="h-56 w-full rounded-xl border bg-white p-3 font-mono text-sm outline-none focus:ring-2 focus:ring-blue-500"
          />
          <div className="mt-4 flex justify-end gap-2">
            <button
              onClick={() => setAddOpen(false)}
              className="rounded-xl border bg-white px-3 py-2 text-sm font-bold"
            >
              取消
            </button>
            <button
              onClick={addFromText}
              className="rounded-xl bg-slate-950 px-3 py-2 text-sm font-bold text-white"
            >
              确认添加
            </button>
          </div>
        </Modal>
      )}
    </>
  );
}

function PluginsPage() {
  const plugins = [
    {
      name: "openai-provider",
      status: "已接入",
      desc: "负责 OpenAI 注册、登录和 token 换取流程。",
    },
    {
      name: "mail-outlook",
      status: "已接入",
      desc: "负责登录 Outlook 邮箱并读取邮件。",
    },
    {
      name: "openai-otp",
      status: "已接入",
      desc: "通用验证码解析插件，负责从邮件 HTML 或正文内容中提取 OpenAI 验证码。",
    },
    {
      name: "proxy-pool",
      status: "已实现",
      desc: "负责代理池、测速和任务代理选择。",
    },
  ];
  return (
    <div className="space-y-4">
      <Card title="插件列表" icon={<PlugZap size={18} />}>
        <p className="text-sm text-slate-500">当前已接入的功能插件。</p>
      </Card>
      <div className="grid gap-3 md:grid-cols-2">
        {plugins.map((p) => (
          <Card key={p.name} title={p.name} icon={<PlugZap size={18} />}>
            <div className="mb-3">
              <Badge status="success" text={p.status} />
            </div>
            <p className="text-sm leading-6 text-slate-500">{p.desc}</p>
          </Card>
        ))}
      </div>
    </div>
  );
}

function JobSummary({
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
          {activeJob && taskItems.length === 0 && (
            <div className="rounded-xl border bg-slate-50 p-3 text-sm text-slate-500">
              本任务暂无邮箱明细。
            </div>
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
function JobDetail({
  job,
  mailboxes,
  openMailboxDetail,
}: {
  job: Job | null;
  mailboxes: Mailbox[];
  openMailboxDetail: (mailbox: Mailbox) => void;
}) {
  return (
    <Card title={job ? `任务详情 #${job.id}` : "任务详情"} className="min-h-0">
      {!job && <p className="text-slate-500">暂无任务。</p>}
      {job && (
        <div className="flex h-[360px] flex-col lg:h-full lg:min-h-0">
          <div className="mb-3 grid grid-cols-3 gap-2">
            <MiniStat label="总数" value={job.total_count} />
            <MiniStat label="成功" value={job.success_count} />
            <MiniStat label="失败" value={job.failed_count} />
          </div>
          <div className="min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
            {(job.items || []).length === 0 && (
              <div className="rounded-xl border bg-slate-50 p-3 text-sm text-slate-500">
                本任务暂无邮箱明细。
              </div>
            )}
            {(job.items || []).map((item) => {
              const mailbox = mailboxes.find(
                (mailbox) => mailbox.email === item.email,
              );
              const canOpen = Boolean(mailbox);
              return (
              <button
                type="button"
                key={item.id}
                onClick={() => mailbox && openMailboxDetail(mailbox)}
                disabled={!canOpen}
                className={`w-full rounded-xl border bg-slate-50 p-3 text-left transition ${
                  canOpen
                    ? "hover:border-blue-200 hover:bg-blue-50/50"
                    : "cursor-default"
                }`}
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="break-all font-bold text-slate-900">
                      {item.email}
                    </div>
                    <div className="mt-1 text-sm text-slate-500">
                      耗时 {item.duration_ms}ms
                    </div>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <Badge status={item.status} text={resultText(item.status)} />
                    {canOpen && (
                      <span className="rounded-full bg-slate-200/70 px-2 py-0.5 text-xs font-bold text-slate-600">
                        详情
                      </span>
                    )}
                  </div>
                </div>
                {item.error && (
                  <div className="mt-2 break-all rounded-lg bg-rose-50 p-2 text-sm text-rose-700">
                    {item.error}
                  </div>
                )}
              </button>
              );
            })}
          </div>
        </div>
      )}
    </Card>
  );
}
function LogPanel({
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
      title={activeJob ? `任务 #${activeJob.id} 邮箱注册实时日志` : "邮箱注册实时日志"}
      icon={<Database size={18} />}
      className="min-h-0"
    >
      <div
        className={`overflow-y-auto rounded-xl border bg-slate-50 p-2 font-mono text-xs text-slate-700 shadow-inner ${
          fillHeight ? "h-[360px] min-h-0 lg:h-full" : "h-[360px]"
        }`}
      >
        {logs.length === 0 && (
          <div className="px-2 py-1.5 text-slate-500">
            暂无日志。
          </div>
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
            <div className="mt-0.5 min-w-0 break-words pl-0 leading-5 text-slate-800">
              {log.message}
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}

function MailboxDetailModal({
  detail,
  detailDraft,
  credentialsOpen,
  busy,
  onClose,
  onToggleCredentials,
  onUpdateDraft,
  onUpdateCredentialLine,
  onSave,
}: {
  detail: Mailbox;
  detailDraft: MailboxUpdate;
  credentialsOpen: boolean;
  busy: boolean;
  onClose: () => void;
  onToggleCredentials: () => void;
  onUpdateDraft: (key: keyof MailboxUpdate, value: string) => void;
  onUpdateCredentialLine: (value: string) => void;
  onSave: () => void;
}) {
  return (
    <Modal
      title="邮箱详情"
      subtitle={detailDraft.email || detail.email}
      onClose={onClose}
    >
      <div className="space-y-3 text-sm">
        <Field label="OpenAI 密码">
          <input
            value={detailDraft.register_password || ""}
            onChange={(event) =>
              onUpdateDraft("register_password", event.target.value)
            }
            className="w-full rounded-xl border bg-white px-3 py-2 font-mono text-sm outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="OpenAI 登录密码"
          />
        </Field>
        <InfoRow
          label="最近任务"
          value={
            detail.last_job_id
              ? `#${detail.last_job_id} ${jobTypeText(detail.last_job_type)} ${resultText(detail.last_job_status)}`
              : "-"
          }
        />
        <InfoRow
          label="失败信息"
          value={detail.last_job_error || detail.last_error || "-"}
        />
        <div>
          <div className="mb-2 font-bold text-slate-600">Token</div>
          <pre className="max-h-56 overflow-auto rounded-xl border bg-slate-50 p-3 text-xs">
            {formatToken(detail.token_json)}
          </pre>
        </div>
        <div className="rounded-xl border bg-slate-50 p-3">
          <button
            type="button"
            onClick={onToggleCredentials}
            className="flex w-full items-center justify-between gap-3 text-left font-bold text-slate-700"
          >
            <span>邮箱凭据</span>
            <span className="text-xs text-slate-500">
              {credentialsOpen ? "收起" : "展开"}
            </span>
          </button>
          {credentialsOpen && (
            <textarea
              value={[
                detailDraft.email || "",
                detailDraft.password || "",
                detailDraft.client_id || "",
                detailDraft.access_token || "",
              ].join("----")}
              onChange={(event) => onUpdateCredentialLine(event.target.value)}
              className="mt-2 h-24 w-full rounded-xl border bg-white p-3 font-mono text-xs outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="email@example.com----password----client_id----refresh_token"
            />
          )}
        </div>
        <div className="flex justify-end gap-2 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="rounded-xl border bg-white px-3 py-2 font-bold"
          >
            取消
          </button>
          <button
            type="button"
            onClick={onSave}
            disabled={busy || !detailDraft.email.trim()}
            className="rounded-xl bg-slate-950 px-3 py-2 font-bold text-white disabled:opacity-50"
          >
            保存
          </button>
        </div>
      </div>
    </Modal>
  );
}

function Modal({
  title,
  subtitle,
  onClose,
  children,
}: {
  title: string;
  subtitle?: string;
  onClose: () => void;
  children: React.ReactNode;
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
function DataTable({
  headers,
  children,
}: {
  headers: string[];
  children: React.ReactNode;
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
function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
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
function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border bg-slate-50 p-2.5">
      <div className="text-xs font-bold text-slate-500">{label}</div>
      <div className="mt-1 break-all font-semibold text-slate-900">{value}</div>
    </div>
  );
}
function Card({
  title,
  icon,
  className = "",
  children,
}: {
  title: string;
  icon?: React.ReactNode;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div
      className={`flex flex-col rounded-2xl border border-slate-200/70 bg-white/80 p-4 shadow-soft backdrop-blur ${className}`}
    >
      <div className="mb-3 flex items-center gap-2 text-base font-extrabold">
        {icon}
        {title}
      </div>
      <div className="min-h-0 flex-1">{children}</div>
    </div>
  );
}
function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-xl border bg-white p-3">
      <div className="text-xs font-bold text-slate-500">{label}</div>
      <div className="mt-0.5 text-xl font-black tracking-tight">{value}</div>
    </div>
  );
}
function MiniStat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-xl border bg-slate-50 px-3 py-2">
      <div className="text-xs font-bold text-slate-500">{label}</div>
      <div className="mt-0.5 text-lg font-black">{value}</div>
    </div>
  );
}
function Badge({ status, text }: { status: string; text: string }) {
  const cls = ["registered", "success", "finished"].includes(status)
    ? "bg-emerald-100 text-emerald-700"
    : ["abnormal", "failed"].includes(status)
      ? "bg-rose-100 text-rose-700"
      : ["registering", "running"].includes(status)
        ? "bg-amber-100 text-amber-700"
        : "bg-blue-100 text-blue-700";
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-bold ${cls}`}>
      {text}
    </span>
  );
}
function Toast({ toast, onClose }: { toast: ToastState; onClose: () => void }) {
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
function jobTypeText(type?: string) {
  return type === "login" ? "登录" : "注册";
}
function jobStatusText(status?: string) {
  return status === "running"
    ? "运行中"
    : status === "finished"
      ? "已完成"
      : status === "stopped"
        ? "已结束"
        : status === "failed"
          ? "失败"
          : status || "-";
}
function resultText(status?: string) {
  return status === "success"
    ? "成功"
    : status === "failed"
      ? "失败"
      : status || "-";
}
function formatToken(token?: string) {
  if (!token) return "暂无 token";
  try {
    return JSON.stringify(JSON.parse(token), null, 2);
  } catch {
    return token;
  }
}
function mailboxImportLine(m: Mailbox) {
  return `${m.email}----${m.password || ""}----${m.access_token || ""}----${m.client_id || ""}`;
}
function isValidProxyURL(value: string) {
  try {
    const parsed = new URL(value);
    return (
      ["http:", "https:", "socks5:", "socks5h:"].includes(parsed.protocol) &&
      Boolean(parsed.hostname)
    );
  } catch {
    return false;
  }
}
createRoot(document.getElementById("root")!).render(<App />);
