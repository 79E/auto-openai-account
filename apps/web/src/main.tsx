import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Navigate, NavLink, Route, Routes, useLocation } from "react-router-dom";
import { CreateTaskModal } from "./components/CreateTaskModal/CreateTaskModal";
import { MailboxDetailModal } from "./components/MailboxDetailModal/MailboxDetailModal";
import { Toast } from "./components/Toast/Toast";
import { TokenExportConfirmModal } from "./components/TokenExportConfirmModal/TokenExportConfirmModal";
import { api, normalizeSettingsPayload } from "./lib/api";
import { appName, emptyStats, nav, routeTitles } from "./lib/constants";
import { canExportJobTokens, downloadJsonFile, formatFileDate } from "./lib/format";
import { JobsPage } from "./pages/JobsPage/JobsPage";
import { MailboxesPage } from "./pages/MailboxesPage/MailboxesPage";
import { Overview } from "./pages/Overview/Overview";
import { PluginsPage } from "./pages/PluginsPage/PluginsPage";
import { ProxyPoolPage } from "./pages/ProxyPoolPage/ProxyPoolPage";
import type { Job, JobTokenExportItem, Mailbox, MailboxUpdate, RuntimeLog, SettingsPayload, Stats, ToastState, TokenExportConfirm } from "./types";
import "./styles.css";
import styles from "./App.module.css";

function App() {
  const location = useLocation();
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
  const [tokenExportConfirm, setTokenExportConfirm] =
    useState<TokenExportConfirm>(null);

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
    const targetId = preferredJobId || activeJob?.id;
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
    const title = routeTitles[location.pathname] || "总览";
    document.title = `${title} - ${appName}`;
  }, [location.pathname]);

  useEffect(() => {
    if (location.pathname !== "/jobs" || activeJob || jobs.length === 0) return;
    loadJob(jobs[0].id).catch(console.error);
  }, [location.pathname, activeJob, jobs]);

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

  async function exportJobTokens(job: Job) {
    if (!canExportJobTokens(job)) return;
    setBusy(true);
    try {
      const result = await api<{ count: number; items: JobTokenExportItem[] }>(
        `/api/register-jobs/${job.id}/tokens`,
      );
      setTokenExportConfirm({
        jobId: job.id,
        count: result.count,
        items: result.items || [],
      });
    } catch (error) {
      showToast(error instanceof Error ? error.message : "导出 token 失败", "error");
    } finally {
      setBusy(false);
    }
  }

  function confirmExportJobTokens() {
    if (!tokenExportConfirm) return;
    const filename = `${formatFileDate(new Date())}_task-${tokenExportConfirm.jobId}_${tokenExportConfirm.count}.json`;
    downloadJsonFile(filename, tokenExportConfirm.items);
    showToast(`已导出 ${tokenExportConfirm.count} 条 token`, "success");
    setTokenExportConfirm(null);
  }

  const registered = stats.mailboxes.registered || 0;
  const abnormal = stats.mailboxes.abnormal || 0;
  const newCount = stats.mailboxes.new || 0;
  const runningCount = stats.mailboxes.registering || 0;
  const loginingCount = stats.mailboxes.logining || 0;

  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top_left,#dbeafe_0,#f8fafc_34%,#eef2ff_100%)] text-slate-950">
      <div className="mx-auto max-w-[92rem] px-4 py-4 sm:px-5">
        <header className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <NavLink
            to="/"
            className="flex items-center gap-2 font-extrabold"
          >
            <img
              src="/logo.svg"
              alt=""
              aria-hidden="true"
              draggable={false}
              className="h-8 w-8"
            />
            {appName}
          </NavLink>
          <nav className="flex w-full gap-1 overflow-x-auto rounded-xl border border-slate-200/70 bg-white/70 p-1 shadow-sm backdrop-blur sm:w-auto">
            {nav.map((item) => (
              <NavLink
                key={item.path}
                to={item.path}
                end={item.end}
                className={({ isActive }) =>
                  isActive
                    ? "shrink-0 rounded-lg bg-white px-3 py-1.5 font-semibold shadow-sm"
                    : "shrink-0 px-3 py-1.5 text-slate-500 transition hover:text-slate-950"
                }
              >
                {item.label}
              </NavLink>
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
        {tokenExportConfirm && (
          <TokenExportConfirmModal
            exportInfo={tokenExportConfirm}
            onClose={() => setTokenExportConfirm(null)}
            onConfirm={confirmExportJobTokens}
          />
        )}
        <Routes>
          <Route
            path="/"
            element={
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
            }
          />
          <Route
            path="/mailboxes"
            element={
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
            }
          />
          <Route
            path="/jobs"
            element={
              <JobsPage
                jobs={jobs}
                activeJob={activeJob}
                logs={logs}
                mailboxes={mailboxes}
                openTask={() => setTaskOpen(true)}
                openMailboxDetail={openMailboxDetail}
                stopTask={stopTask}
                exportJobTokens={exportJobTokens}
                selectJob={loadJob}
                busy={busy}
              />
            }
          />
          <Route
            path="/proxies"
            element={
              settingsDraft ? (
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
              ) : null
            }
          />
          <Route path="/plugins" element={<PluginsPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </div>
    </div>
  );
}

createRoot(document.getElementById("root")!).render(
  <BrowserRouter>
    <App />
  </BrowserRouter>,
);
