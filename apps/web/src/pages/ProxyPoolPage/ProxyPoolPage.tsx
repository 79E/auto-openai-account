import { useState } from "react";
import { PlugZap } from "lucide-react";
import { api } from "../../lib/api";
import { isValidProxyURL } from "../../lib/format";
import type { ProxyTestResult, SettingsPayload } from "../../types";
import { Badge } from "../../components/Badge/Badge";
import { Card } from "../../components/Card/Card";
import { EmptyState } from "../../components/EmptyState/EmptyState";
import { Modal } from "../../components/Modal/Modal";
import styles from "./ProxyPoolPage.module.css";

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
    setTesting((prev) => Array.from(new Set([...prev, proxy])));
    try {
      const d = await api<{ items: ProxyTestResult[] }>("/api/proxy/test", {
        method: "POST",
        body: JSON.stringify({ proxy }),
      });
      setResults((p) => ({ ...p, [proxy]: d.items[0] }));
    } finally {
      setTesting((prev) => prev.filter((item) => item !== proxy));
    }
  }
  async function testAll() {
    const ps = settingsDraft.proxies.filter(Boolean);
    if (!ps.length) return;
    setTesting((prev) => Array.from(new Set([...prev, ...ps])));
    await Promise.all(
      ps.map(async (proxy) => {
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
      }),
    );
  }
  const hasTesting = testing.length > 0;
  return (
    <>
      <Card
        title="代理池"
        icon={<PlugZap size={18} />}
        actions={
          <div className="flex gap-2">
            <button
              onClick={() => setAddOpen(true)}
              className="rounded-xl border bg-white px-3 py-2 text-sm font-bold"
            >
              新增代理
            </button>
            <button
              onClick={testAll}
              disabled={hasTesting}
              className="rounded-xl border bg-white px-3 py-2 text-sm font-bold disabled:opacity-50"
            >
              {hasTesting ? "测试中..." : "测试全部"}
            </button>
          </div>
        }
      >
        <div className="space-y-2">
          {settingsDraft.proxies.map((proxy, i) => {
            const r = results[proxy];
            const isTesting = testing.includes(proxy);
            return (
              <div key={i} className="rounded-xl border bg-slate-50 p-3">
                <div className="flex flex-col gap-2 lg:flex-row lg:items-center">
                  <div className="min-w-0 flex-1 rounded-xl border border-slate-200 bg-white px-3 py-2 font-mono text-sm text-slate-800">
                    <div className="truncate">{proxy}</div>
                  </div>
                  <button
                    onClick={() => test(proxy)}
                    disabled={isTesting}
                    className="rounded-xl bg-slate-950 px-3 py-2 text-sm font-bold text-white disabled:opacity-50"
                  >
                    {isTesting ? "测试中..." : "测速"}
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
          {!settingsDraft.proxies.length && (
            <EmptyState
              title="暂无代理"
              description="点击右上角“新增代理”添加代理后，可在这里测速和管理。"
            />
          )}
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
