import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import {
  ChevronDown,
  KeyRound,
  MessageSquareText,
  Pencil,
  Plus,
  RefreshCw,
  Save,
  Trash2,
  X,
} from "lucide-react";
import { Badge } from "../../components/Badge/Badge";
import { Card } from "../../components/Card/Card";
import { EmptyState } from "../../components/EmptyState/EmptyState";
import { Modal } from "../../components/Modal/Modal";
import { fetchSMSCatalog } from "../../lib/api";
import type {
  SMSCatalog,
  SMSCatalogCountry,
  SMSCatalogService,
  SMSConfig,
  SettingsPayload,
} from "../../types";

const platforms = [
  { value: "smsbower", label: "SMSBower" },
  { value: "hero-sms", label: "Hero SMS" },
];

type SMSConfigForm = Omit<SMSConfig, "max_price"> & {
  max_price: string;
};

function nextConfigName(configs: SMSConfig[]) {
  const names = new Set(configs.map((config) => config.name.trim()));
  let index = configs.length + 1;
  while (names.has(`sms-${index}`)) index += 1;
  return `sms-${index}`;
}

function emptySMSConfig(configs: SMSConfig[]): SMSConfigForm {
  return {
    name: nextConfigName(configs),
    platform: "smsbower",
    api_key: "",
    service_id: "dr",
    country_id: 38,
    max_price: "0",
  };
}

function normalizeSMSConfig(config: SMSConfigForm): SMSConfig {
  return {
    name: config.name.trim(),
    platform: config.platform || "smsbower",
    api_key: config.api_key.trim(),
    service_id: (config.service_id || "dr").trim(),
    country_id: Number(config.country_id),
    max_price: Number(config.max_price),
  };
}

function serviceLabel(service: SMSCatalogService) {
  return service.name ? `${service.name} (${service.code})` : service.code;
}

function countryLabel(country: SMSCatalogCountry) {
  const name = country.chn || country.eng || country.rus || `国家 ${country.id}`;
  return `${name} (${country.id})`;
}

function platformLabel(value: string) {
  return platforms.find((platform) => platform.value === value)?.label || value;
}

function maskAPIKey(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return "未填写";
  if (trimmed.length <= 8) return `${trimmed.slice(0, 2)}****${trimmed.slice(-2)}`;
  return `${trimmed.slice(0, 6)}****${trimmed.slice(-4)}`;
}

function ConfigMetric({
  label,
  value,
  icon,
}: {
  label: string;
  value: string | number;
  icon?: ReactNode;
}) {
  return (
    <div className="min-h-[5.25rem] rounded-xl border border-slate-200/80 bg-white/90 px-3 py-2 shadow-sm">
      <div className="flex items-center gap-1 text-sm font-black text-slate-400">
        {icon}
        {label}
      </div>
      <div className="mt-1 break-all font-mono text-base font-bold text-slate-700">
        {value}
      </div>
    </div>
  );
}

export function SmsSettingsPage({
  settingsDraft,
  setSettingsDraft,
  saveSettings,
  busy,
}: {
  settingsDraft: SettingsPayload;
  setSettingsDraft: (settings: SettingsPayload) => void;
  saveSettings: (settings: SettingsPayload) => Promise<void> | void;
  busy: boolean;
}) {
  const smsConfigs = settingsDraft.sms_configs || [];
  const [form, setForm] = useState<SMSConfigForm>(() => emptySMSConfig(smsConfigs));
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [formError, setFormError] = useState("");
  const [saving, setSaving] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [catalog, setCatalog] = useState<SMSCatalog | null>(null);
  const [catalogLoading, setCatalogLoading] = useState(false);
  const [catalogError, setCatalogError] = useState("");
  const inputClass =
    "mt-1 w-full rounded-xl border bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500";
  const selectClass = `${inputClass} appearance-none pr-10`;
  const isEditing = editingIndex !== null;
  const submitText = isEditing ? "保存配置" : "添加配置";

  const configNames = useMemo(
    () => smsConfigs.map((config) => config.name.trim()).filter(Boolean),
    [smsConfigs],
  );

  const serviceOptions = useMemo(() => {
    const services = catalog?.services || [];
    const current = form.service_id || "dr";
    if (!services.some((service) => service.code === current)) {
      return [{ code: current, name: "当前服务" }, ...services];
    }
    return services;
  }, [catalog?.services, form.service_id]);

  const countryOptions = useMemo(() => {
    const countries = catalog?.countries || [];
    const current = Number(form.country_id) || 38;
    if (!countries.some((country) => country.id === current)) {
      return [{ id: current, chn: "当前国家/地区" }, ...countries];
    }
    return countries;
  }, [catalog?.countries, form.country_id]);

  function updateForm(updates: Partial<SMSConfigForm>) {
    setForm((current) => ({ ...current, ...updates }));
    setFormError("");
    if ("platform" in updates || "api_key" in updates) {
      setCatalog(null);
      setCatalogError("");
    }
  }

  function toFormSMSConfig(config: SMSConfig): SMSConfigForm {
    return {
      ...config,
      max_price: String(config.max_price ?? 0),
    };
  }

  function resetForm(nextConfigs = smsConfigs) {
    setForm(emptySMSConfig(nextConfigs));
    setEditingIndex(null);
    setFormError("");
  }

  function openCreateForm() {
    setForm(emptySMSConfig(smsConfigs));
    setEditingIndex(null);
    setFormError("");
    setCatalog(null);
    setCatalogError("");
    setFormOpen(true);
  }

  function closeForm() {
    if (saving) return;
    setFormOpen(false);
    setCatalog(null);
    setCatalogError("");
    resetForm();
  }

  function validateForm() {
    const normalized = normalizeSMSConfig(form);
    if (!normalized.name) return "请输入配置名称";
    if (normalized.platform !== "smsbower" && normalized.platform !== "hero-sms") {
      return "请选择支持的 SMS 平台";
    }
    if (!normalized.api_key) return "请输入 API 密钥";
    if (!normalized.service_id) return "请选择服务";
    const countryID = Number(form.country_id);
    const maxPrice = Number(form.max_price);
    if (!Number.isFinite(countryID) || countryID <= 0) {
      return "请选择国家/地区";
    }
    if (!Number.isFinite(maxPrice) || maxPrice < 0) {
      return "最高价格不能小于 0";
    }
    const duplicate = configNames.some((name, index) => {
      if (editingIndex !== null && index === editingIndex) return false;
      return name === normalized.name;
    });
    if (duplicate) return "配置名称不能重复";
    return "";
  }

  function persist(nextConfigs: SMSConfig[]) {
    const next = { ...settingsDraft, sms_configs: nextConfigs };
    setSettingsDraft(next);
    return saveSettings(next);
  }

  async function submitForm() {
    const error = validateForm();
    if (error) {
      setFormError(error);
      return;
    }
    const normalized = normalizeSMSConfig(form);
    const nextConfigs =
      editingIndex === null
        ? [...smsConfigs, normalized]
        : smsConfigs.map((config, index) =>
            index === editingIndex ? normalized : config,
          );
    setSaving(true);
    try {
      await persist(nextConfigs);
      setFormOpen(false);
      resetForm(nextConfigs);
    } finally {
      setSaving(false);
    }
  }

  async function removeConfig(index: number) {
    const nextConfigs = smsConfigs.filter((_, currentIndex) => currentIndex !== index);
    setSaving(true);
    try {
      await persist(nextConfigs);
      if (editingIndex === index) {
        resetForm(nextConfigs);
      } else if (editingIndex !== null && index < editingIndex) {
        setEditingIndex(editingIndex - 1);
      }
    } finally {
      setSaving(false);
    }
  }

  function startEdit(index: number) {
    setEditingIndex(index);
    setForm(toFormSMSConfig(smsConfigs[index]));
    setFormError("");
    setCatalog(null);
    setCatalogError("");
    setFormOpen(true);
  }

  async function loadCatalog() {
    const apiKey = form.api_key.trim();
    if (!apiKey) {
      setCatalogError("请先输入 API 密钥");
      return;
    }
    setCatalogLoading(true);
    setCatalogError("");
    try {
      const result = await fetchSMSCatalog(form.platform || "smsbower", apiKey);
      setCatalog({
        services: result.services || [],
        countries: result.countries || [],
      });
    } catch (error) {
      setCatalogError(error instanceof Error ? error.message : "获取列表失败");
    } finally {
      setCatalogLoading(false);
    }
  }

  return (
    <>
      <Card
        title="SMS 配置"
        icon={<MessageSquareText size={18} />}
        actions={
          <button
            type="button"
            onClick={openCreateForm}
            className="inline-flex items-center gap-2 rounded-xl border bg-white px-3 py-2 text-sm font-bold"
          >
            <Plus size={16} />
            新增配置
          </button>
        }
      >
        <div className="grid gap-3 lg:grid-cols-2">
          {smsConfigs.length === 0 && (
            <div className="lg:col-span-2">
              <EmptyState
                title="暂无 SMS 配置"
                description="Codex 授权登录任务需要先保存一条 SMS 配置。"
              />
            </div>
          )}
          {smsConfigs.map((config, index) => (
            <div
              key={`${config.name}-${index}`}
              className="flex h-full flex-col rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4 shadow-sm transition hover:border-blue-200 hover:bg-white/80"
            >
              <div className="flex h-full flex-col gap-4">
                <div className="min-w-0 flex-1">
                  <div className="mb-3 flex min-w-0 flex-wrap items-center gap-2">
                    <div className="truncate text-lg font-black text-slate-950">
                      {config.name || "-"}
                    </div>
                    <span className="rounded-full bg-emerald-100 px-3 py-1 text-sm font-black text-emerald-700">
                      {platformLabel(config.platform)}
                    </span>
                    {editingIndex === index && (
                      <Badge status="running" text="编辑中" />
                    )}
                  </div>
                  <div className="grid gap-2 sm:grid-cols-2">
                    <ConfigMetric label="服务" value={config.service_id || "dr"} />
                    <ConfigMetric label="国家/地区" value={config.country_id || 38} />
                    <ConfigMetric label="最高价格" value={config.max_price || 0} />
                    <ConfigMetric
                      label="API 密钥"
                      value={maskAPIKey(config.api_key || "")}
                      icon={<KeyRound size={15} />}
                    />
                  </div>
                </div>
                <div className="flex shrink-0 flex-wrap justify-end gap-2">
                  <button
                    type="button"
                    onClick={() => startEdit(index)}
                    className="inline-flex h-11 items-center gap-2 rounded-xl border border-slate-200 bg-white px-4 text-sm font-black shadow-sm"
                  >
                    <Pencil size={16} />
                    编辑
                  </button>
                  <button
                    type="button"
                    disabled={busy || saving}
                    onClick={() => removeConfig(index)}
                    className="inline-flex h-11 items-center gap-2 rounded-xl border border-rose-200 bg-rose-50 px-4 text-sm font-black text-rose-700 shadow-sm disabled:opacity-50"
                  >
                    <Trash2 size={16} />
                    删除
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      </Card>
      {formOpen && (
        <Modal
          title={isEditing ? "编辑 SMS 配置" : "新增 SMS 配置"}
          subtitle="Hero SMS / SMSBower"
          onClose={closeForm}
        >
          <div className="grid gap-3 md:grid-cols-2">
            <label className="text-sm font-bold text-slate-600">
              配置名称
              <input
                className={inputClass}
                value={form.name}
                placeholder="sms-codex-us"
                autoComplete="off"
                onChange={(event) => updateForm({ name: event.target.value })}
              />
            </label>
            <label className="text-sm font-bold text-slate-600">
              短信平台
              <span className="relative block">
                <select
                  className={selectClass}
                  value={form.platform || "smsbower"}
                  onChange={(event) =>
                    updateForm({ platform: event.target.value })
                  }
                >
                  {platforms.map((platform) => (
                    <option key={platform.value} value={platform.value}>
                      {platform.label}
                    </option>
                  ))}
                </select>
                <ChevronDown
                  size={16}
                  className="pointer-events-none absolute right-3 top-1/2 mt-0.5 -translate-y-1/2 text-slate-400"
                />
              </span>
            </label>
            <label className="text-sm font-bold text-slate-600 md:col-span-2">
              API 密钥
              <input
                className={`${inputClass} font-mono`}
                value={form.api_key}
                placeholder="粘贴短信平台 API 密钥"
                autoComplete="off"
                onChange={(event) => updateForm({ api_key: event.target.value })}
              />
            </label>
            <div className="md:col-span-2">
              <button
                type="button"
                onClick={loadCatalog}
                disabled={catalogLoading || !form.api_key.trim()}
                className="inline-flex items-center gap-2 rounded-xl border bg-white px-3 py-2 text-sm font-bold disabled:opacity-50"
              >
                <RefreshCw size={16} />
                {catalogLoading ? "获取中..." : "获取服务和国家"}
              </button>
              {catalog && (
                <span className="ml-3 text-sm font-semibold text-slate-500">
                  已加载 {catalog.services.length} 个服务，{catalog.countries.length} 个国家/地区
                </span>
              )}
              {catalogError && (
                <div className="mt-2 rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm font-semibold text-rose-700">
                  {catalogError}
                </div>
              )}
            </div>
            <label className="text-sm font-bold text-slate-600">
              服务
              <span className="relative block">
                <select
                  className={selectClass}
                  value={form.service_id || "dr"}
                  onChange={(event) =>
                    updateForm({ service_id: event.target.value })
                  }
                >
                  {serviceOptions.map((service) => (
                    <option key={service.code} value={service.code}>
                      {serviceLabel(service)}
                    </option>
                  ))}
                </select>
                <ChevronDown
                  size={16}
                  className="pointer-events-none absolute right-3 top-1/2 mt-0.5 -translate-y-1/2 text-slate-400"
                />
              </span>
            </label>
            <label className="text-sm font-bold text-slate-600">
              国家/地区
              <span className="relative block">
                <select
                  className={selectClass}
                  value={form.country_id}
                  onChange={(event) =>
                    updateForm({ country_id: Number(event.target.value) })
                  }
                >
                  {countryOptions.map((country) => (
                    <option key={country.id} value={country.id}>
                      {countryLabel(country)}
                    </option>
                  ))}
                </select>
                <ChevronDown
                  size={16}
                  className="pointer-events-none absolute right-3 top-1/2 mt-0.5 -translate-y-1/2 text-slate-400"
                />
              </span>
            </label>
<label className="text-sm font-bold text-slate-600 md:col-span-2">
                最高价格
                <input
                  className={inputClass}
                  type="number"
                  min={0}
                  step="0.0001"
                  value={form.max_price}
                  placeholder={"0"}
                  onChange={(event) => {
                    const raw = event.target.value;
                    if (raw === "") {
                      updateForm({ max_price: "" });
                      return;
                    }
                    if (/^(\d+)?(\.\d*)?$/.test(raw)) {
                      updateForm({ max_price: raw });
                    }
                  }}
                />
              </label>
          </div>
          {formError && (
            <div className="mt-3 rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm font-semibold text-rose-700">
              {formError}
            </div>
          )}
          <div className="mt-4 flex flex-wrap justify-end gap-2">
            <button
              type="button"
              onClick={closeForm}
              disabled={saving}
              className="inline-flex items-center gap-2 rounded-xl border bg-white px-3 py-2 text-sm font-bold disabled:opacity-50"
            >
              <X size={16} />
              取消
            </button>
            <button
              type="button"
              onClick={submitForm}
              disabled={busy || saving}
              className="inline-flex items-center gap-2 rounded-xl bg-slate-950 px-3 py-2 text-sm font-bold text-white disabled:opacity-50"
            >
              {isEditing ? <Save size={16} /> : <Plus size={16} />}
              {busy || saving ? "保存中..." : submitText}
            </button>
          </div>
        </Modal>
      )}
    </>
  );
}
