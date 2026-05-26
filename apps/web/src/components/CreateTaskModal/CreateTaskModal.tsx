import { useState } from "react";
import { defaultPassword } from "../../lib/constants";
import type { Mailbox, SettingsPayload } from "../../types";
import { Field } from "../Field/Field";
import styles from "./CreateTaskModal.module.css";

export function CreateTaskModal({
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
          <Field label="代理模式">
            <select
              className={`${styles.input} ${styles.selectInput}`}
              value={draft.proxy_mode}
              onChange={(e) =>
                setDraft({ ...draft, proxy_mode: e.target.value })
              }
            >
              <option value="local">本地网络（不使用代理）</option>
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
        {taskType === "login" && (
          <div className="mt-3 grid gap-3 md:grid-cols-1">
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
