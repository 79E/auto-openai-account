import { useState } from "react";
import { PlugZap } from "lucide-react";
import { api, createSettingsItemID } from "../../lib/api";
import { isValidProxyURL } from "../../lib/format";
import type { ProxyGroup, ProxyTestResult, SettingsPayload } from "../../types";
import { Badge } from "../../components/Badge/Badge";
import { Card } from "../../components/Card/Card";
import { EmptyState } from "../../components/EmptyState/EmptyState";
import { Field } from "../../components/Field/Field";
import { Modal } from "../../components/Modal/Modal";
import styles from "./ProxyPoolPage.module.css";

type AddDraft = {
  name: string;
  mode: "random" | "round_robin";
  text: string;
};

const initialAddDraft: AddDraft = {
  name: "",
  mode: "round_robin",
  text: "",
};

export function ProxyPoolPage({
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
  const [testing, setTesting] = useState<string[]>([]);
  const [addOpen, setAddOpen] = useState(false);
  const [addDraft, setAddDraft] = useState<AddDraft>(initialAddDraft);
  const [editingGroupID, setEditingGroupID] = useState<string | null>(null);

  const groups = settingsDraft.proxy_groups || [];

  function persist(proxyGroups: ProxyGroup[]) {
    const next = {
      ...settingsDraft,
      proxy_groups: proxyGroups,
    };
    setSettingsDraft(next);
    saveSettings(next);
  }

  function resetAddDraft() {
    setAddDraft(initialAddDraft);
    setEditingGroupID(null);
  }

  function removeGroup(id: string) {
    persist(groups.filter((group) => group.id !== id));
  }

  function removeProxy(groupID: string, proxy: string) {
    const nextGroups = groups
      .map((group) =>
        group.id !== groupID
          ? group
          : {
              ...group,
              proxies: group.proxies.filter((item) => item !== proxy),
            },
      )
      .filter((group) => group.proxies.length > 0);
    persist(nextGroups);
  }

  function openAddModal() {
    resetAddDraft();
    setAddOpen(true);
  }

  function openEditModal(group: ProxyGroup) {
    setEditingGroupID(group.id);
    setAddDraft({
      name: group.name,
      mode: group.mode === "random" ? "random" : "round_robin",
      text: group.proxies.join("\n"),
    });
    setAddOpen(true);
  }

  function saveGroup() {
    const name = addDraft.name.trim();
    if (!name) {
      showToast("请输入分组名", "error");
      return;
    }
    if (
      groups.some(
        (group) =>
          group.name.trim().toLowerCase() === name.toLowerCase() &&
          group.id !== editingGroupID,
      )
    ) {
      showToast(`分组名已存在：${name}`, "error");
      return;
    }
    const proxies = Array.from(
      new Set(
        addDraft.text
          .split("\n")
          .map((item) => item.trim())
          .filter(Boolean),
      ),
    );
    if (!proxies.length) {
      showToast("请至少输入一条代理", "error");
      return;
    }
    const invalid = proxies.find((item) => !isValidProxyURL(item));
    if (invalid) {
      showToast(`代理格式不正确：${invalid}`, "error");
      return;
    }
    const nextGroup = {
      id: editingGroupID || createSettingsItemID(),
      name,
      mode: addDraft.mode,
      proxies,
    };
    if (editingGroupID) {
      persist(
        groups.map((group) =>
          group.id === editingGroupID ? nextGroup : group,
        ),
      );
    } else {
      persist([...groups, nextGroup]);
    }
    resetAddDraft();
    setAddOpen(false);
  }

  async function testProxy(proxy: string) {
    if (!proxy.trim()) return;
    setTesting((prev) => Array.from(new Set([...prev, proxy])));
    try {
      const d = await api<{ items: ProxyTestResult[] }>("/api/proxy/test", {
        method: "POST",
        body: JSON.stringify({ proxy }),
      });
      setResults((prev) => ({ ...prev, [proxy]: d.items[0] }));
    } catch (error) {
      setResults((prev) => ({
        ...prev,
        [proxy]: {
          proxy,
          ok: false,
          latency_ms: 0,
          error: error instanceof Error ? error.message : "测试失败",
        },
      }));
    } finally {
      setTesting((prev) => prev.filter((item) => item !== proxy));
    }
  }

  async function testGroup(group: ProxyGroup) {
    const proxies = group.proxies.filter(Boolean);
    if (!proxies.length) return;
    setTesting((prev) => Array.from(new Set([...prev, ...proxies])));
    await Promise.all(proxies.map((proxy) => testProxy(proxy)));
  }

  return (
    <>
      <Card
        title="代理池"
        icon={<PlugZap size={18} />}
        actions={
          <button
            onClick={openAddModal}
            className="rounded-xl border bg-white px-3 py-2 text-sm font-bold"
          >
            新增代理
          </button>
        }
      >
        <div className="space-y-4">
          {groups.map((group) => {
            const groupBusy = group.proxies.some((proxy) => testing.includes(proxy));
            return (
              <div key={group.id} className="rounded-2xl border border-slate-200 bg-slate-50/80 p-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="text-base font-black text-slate-900">{group.name}</h3>
                      <Badge
                        status={group.mode === "round_robin" ? "running" : "success"}
                        text={group.mode === "round_robin" ? "轮询" : "随机"}
                      />
                      <span className="text-sm text-slate-500">{group.proxies.length} 条代理</span>
                    </div>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <button
                      onClick={() => openEditModal(group)}
                      className="rounded-xl border bg-white px-3 py-2 text-sm font-bold"
                    >
                      编辑分组
                    </button>
                    <button
                      onClick={() => testGroup(group)}
                      disabled={groupBusy}
                      className="rounded-xl border bg-white px-3 py-2 text-sm font-bold disabled:opacity-50"
                    >
                      {groupBusy ? "测试中..." : "测试全部"}
                    </button>
                    <button
                      onClick={() => removeGroup(group.id)}
                      className="rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm font-bold text-rose-700"
                    >
                      删除分组
                    </button>
                  </div>
                </div>
                <div className="mt-3 space-y-2">
                  {group.proxies.map((proxy) => {
                    const result = results[proxy];
                    const isTesting = testing.includes(proxy);
                    return (
                      <div key={`${group.id}-${proxy}`} className="rounded-xl border border-slate-200 bg-white p-3">
                        <div className="flex flex-col gap-2 lg:flex-row lg:items-center">
                          <div className="min-w-0 flex-1 rounded-xl border border-slate-200 bg-slate-50 px-3 py-2 font-mono text-sm text-slate-800">
                            <div className="truncate">{proxy}</div>
                          </div>
                          <button
                            onClick={() => testProxy(proxy)}
                            disabled={isTesting}
                            className="rounded-xl bg-slate-950 px-3 py-2 text-sm font-bold text-white disabled:opacity-50"
                          >
                            {isTesting ? "测试中..." : "测速"}
                          </button>
                          <button
                            onClick={() => removeProxy(group.id, proxy)}
                            className="rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm font-bold text-rose-700"
                          >
                            删除
                          </button>
                        </div>
                        {result && (
                          <div className="mt-2 rounded-xl border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-600">
                            <div className="flex flex-wrap items-center gap-2">
                              <Badge
                                status={result.ok ? "success" : "failed"}
                                text={result.ok ? "可用" : "失败"}
                              />
                              <span>IP：{result.ip || "-"}</span>
                              <span>延迟：{result.latency_ms}ms</span>
                              {result.error && (
                                <span className="text-rose-600">{result.error}</span>
                              )}
                            </div>
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            );
          })}
          {!groups.length && (
            <EmptyState
              title="暂无代理分组"
              description="点击右上角“新增代理”创建分组后，可按分组测速和管理。"
            />
          )}
        </div>
      </Card>
      {addOpen && (
        <Modal
          title={editingGroupID ? "编辑分组" : "新增代理"}
          subtitle="填写分组名、执行模式和代理列表，支持 http、https、socks5、socks5h"
          onClose={() => {
            setAddOpen(false);
            resetAddDraft();
          }}
        >
          <div className="space-y-3">
            <Field label="分组名">
              <input
                value={addDraft.name}
                onChange={(e) =>
                  setAddDraft((prev) => ({ ...prev, name: e.target.value }))
                }
                placeholder="例如：美国住宅池"
                className={styles.input}
              />
            </Field>
            <Field label="执行模式">
              <select
                value={addDraft.mode}
                onChange={(e) =>
                  setAddDraft((prev) => ({
                    ...prev,
                    mode: e.target.value === "random" ? "random" : "round_robin",
                  }))
                }
                className={`${styles.input} ${styles.selectInput}`}
              >
                <option value="round_robin">轮询</option>
                <option value="random">随机</option>
              </select>
            </Field>
            <Field label="代理列表">
              <textarea
                value={addDraft.text}
                onChange={(e) =>
                  setAddDraft((prev) => ({ ...prev, text: e.target.value }))
                }
                placeholder="socks5://127.0.0.1:7890\nhttp://127.0.0.1:8080"
                className={`${styles.input} h-56 font-mono`}
              />
            </Field>
          </div>
          <div className="mt-4 flex justify-end gap-2">
            <button
              onClick={() => {
                setAddOpen(false);
                resetAddDraft();
              }}
              className="rounded-xl border bg-white px-3 py-2 text-sm font-bold"
            >
              取消
            </button>
            <button
              onClick={saveGroup}
              className="rounded-xl bg-slate-950 px-3 py-2 text-sm font-bold text-white"
            >
              {editingGroupID ? "保存分组" : "确认添加"}
            </button>
          </div>
        </Modal>
      )}
    </>
  );
}
