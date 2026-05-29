import { useState } from "react";
import { defaultPassword } from "../../lib/constants";
import type { Mailbox, SettingsPayload } from "../../types";
import { Field } from "../Field/Field";
import styles from "./CreateTaskModal.module.css";

type TaskFlow =
  | "register_login"
  | "register_codex"
  | "login"
  | "codex_login";

export function CreateTaskModal({
  settings,
  mailboxes,
  busy,
  codexLoginTargetIds,
  loginTargetIds,
  onClose,
  onCreateRegister,
  onCreateLogin,
}: {
  settings: SettingsPayload;
  mailboxes: Mailbox[];
  busy: boolean;
  codexLoginTargetIds?: number[];
  loginTargetIds?: number[];
  onClose: () => void;
  onCreateRegister: (
    settings: SettingsPayload,
    count: number,
    flow: TaskFlow,
    smsConfigID: string,
    proxyGroupID: string,
  ) => void;
  onCreateLogin: (
    settings: SettingsPayload,
    ids: number[],
    flow: TaskFlow,
    smsConfigID: string,
    proxyGroupID: string,
  ) => void;
}) {
  const unused = mailboxes.filter((item) => item.status === "new");
  const used = mailboxes.filter(
    (item) => item.status !== "new" && Boolean(item.register_password || item.password),
  );
  const codexTargetIds = codexLoginTargetIds || [];
  const loginTargetIdsResolved = loginTargetIds || [];
  const forcedCodexLogin = codexTargetIds.length > 0;
  const forcedLogin = loginTargetIdsResolved.length > 0;
  const [flow, setFlow] = useState<TaskFlow>(
    forcedCodexLogin ? "codex_login" : forcedLogin ? "login" : "register_login",
  );
  const [draft, setDraft] = useState<SettingsPayload>({
    ...settings,
    fixed_password: settings.fixed_password || defaultPassword,
    sms_configs: settings.sms_configs || [],
    proxy_groups: settings.proxy_groups || [],
  });
  const [count, setCount] = useState(Math.min(1, unused.length));
  const [loginFilter, setLoginFilter] = useState("used");
  const [smsConfigID, setSMSConfigID] = useState(
    draft.sms_configs[0]?.id || "",
  );
  const [proxyTarget, setProxyTarget] = useState("");
  const isRegisterFlow = ["register_login", "register_codex"].includes(flow);
  const isCodexFlow = flow === "register_codex" || flow === "codex_login";
  const loginCandidates = forcedCodexLogin
    ? used.filter((item) => codexTargetIds.includes(item.id))
    : forcedLogin
      ? used.filter((item) => loginTargetIdsResolved.includes(item.id))
    : used.filter((item) => loginFilter === "used" || item.status === loginFilter);
  const selectedSMSExists =
    !isCodexFlow ||
    draft.sms_configs.some((config) => config.id === smsConfigID);

  function submit() {
    if (isRegisterFlow) {
      onCreateRegister(
        draft,
        Math.max(1, Math.min(count, unused.length)),
        flow,
        smsConfigID,
        proxyTarget,
      );
      return;
    }
    onCreateLogin(
      draft,
      loginCandidates.map((item) => item.id),
      flow,
      smsConfigID,
      proxyTarget,
    );
  }

  const flowOptions: { value: TaskFlow; label: string }[] = forcedCodexLogin
    ? [{ value: "codex_login", label: "Codex 授权登录" }]
    : forcedLogin
      ? [{ value: "login", label: "普通登录" }]
    : [
        { value: "register_login", label: "注册 + 普通登录" },
        { value: "register_codex", label: "注册 + 普通登录 + Codex 授权登录" },
        { value: "login", label: "普通登录" },
        { value: "codex_login", label: "Codex 授权登录" },
      ];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/40 p-3 backdrop-blur-sm">
      <div className="w-full max-w-3xl rounded-2xl border bg-white p-4 shadow-soft">
        <div className="mb-4 flex items-center justify-between">
          <div>
            <h2 className="text-lg font-black">
              {forcedCodexLogin
                ? "创建 Codex 授权登录任务"
                : forcedLogin
                  ? "创建普通登录任务"
                  : "创建任务"}
            </h2>
            <p className="mt-1 text-sm text-slate-500">
              {forcedCodexLogin
                ? "为当前勾选邮箱选择 SMS 配置和代理来源后，直接创建 Codex 授权登录任务。"
                : forcedLogin
                  ? "为当前勾选邮箱选择并发数量和代理来源后，直接创建普通登录任务。"
                : "选择任务类型，并配置本次任务参数。"}
            </p>
          </div>
          <button
            onClick={onClose}
            className="self-start pt-0.5 text-slate-400 hover:text-slate-600"
          >
            ✕
          </button>
        </div>
        {!forcedCodexLogin && !forcedLogin && (
          <div className="mb-3 flex flex-wrap gap-2">
            {flowOptions.map((option) => (
              <button
                key={option.value}
                type="button"
                onClick={() => setFlow(option.value)}
                className={
                  flow === option.value
                    ? "rounded-xl bg-slate-950 px-3 py-1.5 font-bold text-white"
                    : "rounded-xl border bg-white px-3 py-1.5 font-bold"
                }
              >
                {option.label}
              </button>
            ))}
          </div>
        )}
        <div className="grid gap-3 md:grid-cols-2">
          <Field label="并发数量">
            <input
              className={styles.input}
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
          <Field label="代理选择">
            <select
              className={`${styles.input} ${styles.selectInput}`}
              value={proxyTarget}
              onChange={(e) => setProxyTarget(e.target.value)}
            >
              <option value="">本地网络（直接请求）</option>
              {draft.proxy_groups.map((group) => (
                <option key={group.id} value={group.id}>
                  {group.name} · {group.mode === "round_robin" ? "轮询" : "随机"}
                </option>
              ))}
            </select>
          </Field>
        </div>
        {isRegisterFlow && (
          <div className="mt-3 grid gap-3 md:grid-cols-2">
            <Field label={`注册数量（可用未使用邮箱 ${unused.length} 个）`}>
              <input
                className={styles.input}
                type="number"
                min={1}
                max={unused.length}
                value={count}
                onChange={(e) => setCount(Number(e.target.value))}
              />
            </Field>
            <Field label="密码模式">
              <select
                className={`${styles.input} ${styles.selectInput}`}
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
                  className={styles.input}
                  value={draft.fixed_password || defaultPassword}
                  onChange={(e) =>
                    setDraft({ ...draft, fixed_password: e.target.value })
                  }
                />
              </Field>
            )}
          </div>
        )}
        {(!isRegisterFlow || forcedCodexLogin || forcedLogin) && (
          <div className="mt-3 grid gap-3 md:grid-cols-1">
            {!forcedCodexLogin && !forcedLogin && (
              <Field label="邮箱状态筛选">
                <select
                  className={`${styles.input} ${styles.selectInput}`}
                  value={loginFilter}
                  onChange={(e) => setLoginFilter(e.target.value)}
                >
                  <option value="used">全部已使用</option>
                  <option value="registered">已注册</option>
                  <option value="abnormal">异常</option>
                </select>
              </Field>
            )}
            <p className="text-sm text-slate-500">
              <span className="font-bold text-slate-800">{loginCandidates.length}</span>{" "}
              {forcedCodexLogin
                ? "个已勾选邮箱将创建 Codex 授权登录任务"
                : forcedLogin
                  ? "个已勾选邮箱将创建普通登录任务"
                  : "个邮箱将创建登录任务"}
            </p>
          </div>
        )}
        {isCodexFlow && (
          <div className="mt-3">
            <Field label="SMS 配置">
              <select
                className={`${styles.input} ${styles.selectInput}`}
                value={smsConfigID}
                onChange={(e) => setSMSConfigID(e.target.value)}
              >
                <option value="">请选择 SMS 配置</option>
                {draft.sms_configs.map((config) => (
                  <option key={config.id} value={config.id}>
                    {config.name} · {config.platform}
                  </option>
                ))}
              </select>
              {!selectedSMSExists && (
                <p className="mt-2 text-sm font-semibold text-rose-600">
                  Codex 流程必须选择有效的 SMS 配置。
                </p>
              )}
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
              (isRegisterFlow ? unused.length === 0 : loginCandidates.length === 0) ||
              !selectedSMSExists
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
