import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import { Fragment } from "react";
import {
  ChevronDown,
  Database,
  FileDigit,
  KeyRound,
  MessageSquareText,
  Pencil,
  Plus,
  RefreshCw,
  Save,
  Trash2,
  Upload,
  X,
} from "lucide-react";
import { Badge } from "../../components/Badge/Badge";
import { Card } from "../../components/Card/Card";
import { EmptyState } from "../../components/EmptyState/EmptyState";
import { Modal } from "../../components/Modal/Modal";
import {
  createSettingsItemID,
  deletePhonePoolItem,
  fetchPhonePoolItems,
  fetchSMSCatalog,
  importPhonePoolItems,
  previewPhonePoolSMS,
} from "../../lib/api";
import type {
  PhonePoolItem,
  SaveSettingsOptions,
  SMSCatalog,
  SMSCatalogCountry,
  SMSCatalogService,
  SMSConfig,
  SettingsPayload,
} from "../../types";

const providerPlatforms = [
  { value: "smsbower", label: "SMSBower" },
  { value: "hero-sms", label: "Hero SMS" },
];

const configTypes = [
  { value: "provider", label: "平台短信" },
  { value: "pool", label: "自定义号池" },
];

const disableOnErrorOptions = [
  { value: "permanent_only", label: "仅永久错误停用" },
  { value: "any_failure", label: "任意失败停用" },
];

type SMSConfigForm = {
  id: string;
  name: string;
  type: "provider" | "pool";
  platform: string;
  platform_label: string;
  api_key: string;
  service_id: string;
  country_id: number;
  max_price: string;
  max_usage_per_phone: string;
  disable_on_error: "permanent_only" | "any_failure";
};

function nextConfigName(configs: SMSConfig[]) {
  const names = new Set(configs.map((config) => config.name.trim()));
  let index = configs.length + 1;
  while (names.has(`sms-${index}`)) index += 1;
  return `sms-${index}`;
}

function emptySMSConfig(configs: SMSConfig[]): SMSConfigForm {
  return {
    id: createSettingsItemID(),
    name: nextConfigName(configs),
    type: "provider",
    platform: "smsbower",
    platform_label: "",
    api_key: "",
    service_id: "dr",
    country_id: 38,
    max_price: "0",
    max_usage_per_phone: "1",
    disable_on_error: "permanent_only",
  };
}

function normalizeSMSConfig(form: SMSConfigForm): SMSConfig {
  return {
    id: form.id || createSettingsItemID(),
    name: form.name.trim(),
    type: form.type,
    platform: form.type === "pool" ? "custom" : form.platform || "smsbower",
    platform_label: form.platform_label.trim(),
    api_key: form.api_key.trim(),
    service_id: (form.service_id || "dr").trim(),
    country_id: Number(form.country_id) || 38,
    max_price: Number(form.max_price) || 0,
    max_usage_per_phone: Math.max(1, Number(form.max_usage_per_phone) || 1),
    disable_on_error:
      form.disable_on_error === "any_failure" ? "any_failure" : "permanent_only",
  };
}

function serviceLabel(service: SMSCatalogService) {
  return service.name ? `${service.name} (${service.code})` : service.code;
}

function countryLabel(country: SMSCatalogCountry) {
  const name = country.chn || country.eng || country.rus || `国家 ${country.id}`;
  return `${name} (${country.id})`;
}

function platformLabel(config: SMSConfig) {
  if (config.type === "pool") {
    return config.platform_label?.trim() || "自定义号池";
  }
  return providerPlatforms.find((platform) => platform.value === config.platform)?.label || config.platform;
}

function configTypeLabel(type: SMSConfig["type"]) {
  return type === "pool" ? "自定义号池" : "平台短信";
}

function maskAPIKey(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return "未填写";
  if (trimmed.length <= 8) return `${trimmed.slice(0, 2)}****${trimmed.slice(-2)}`;
  return `${trimmed.slice(0, 6)}****${trimmed.slice(-4)}`;
}

function phonePoolStatusBadge(status: string) {
  switch (status) {
    case "ready":
      return { status: "success", text: "可用" };
    case "reserved":
      return { status: "running", text: "使用中" };
    case "used_up":
      return { status: "finished", text: "已用尽" };
    case "disabled":
      return { status: "failed", text: "已禁用" };
    case "error":
      return { status: "abnormal", text: "异常" };
    default:
      return { status: "new", text: status || "未知" };
  }
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
  saveSettings: (settings: SettingsPayload, options?: SaveSettingsOptions) => Promise<void> | void;
  busy: boolean;
}) {
  const smsConfigs = settingsDraft.sms_configs || [];
  const [form, setForm] = useState<SMSConfigForm>(() => emptySMSConfig(smsConfigs));
  const [editingConfigID, setEditingConfigID] = useState<string | null>(null);
  const [formError, setFormError] = useState("");
  const [saving, setSaving] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [catalog, setCatalog] = useState<SMSCatalog | null>(null);
  const [catalogLoading, setCatalogLoading] = useState(false);
  const [catalogError, setCatalogError] = useState("");
  const [importOpen, setImportOpen] = useState(false);
  const [importConfig, setImportConfig] = useState<SMSConfig | null>(null);
  const [importText, setImportText] = useState("");
  const [importing, setImporting] = useState(false);
  const [deletingItemID, setDeletingItemID] = useState<number | null>(null);
  const [previewingItemID, setPreviewingItemID] = useState<number | null>(null);
  const [smsPreviewByItemID, setSMSPreviewByItemID] = useState<
    Record<number, { found: boolean; code: string; previewText: string }>
  >({});
  const [importError, setImportError] = useState("");
  const [poolPreview, setPoolPreview] = useState<PhonePoolItem[]>([]);
  const [poolLoading, setPoolLoading] = useState(false);
  const [maxUseSyncOpen, setMaxUseSyncOpen] = useState(false);
  const [pendingConfigs, setPendingConfigs] = useState<SMSConfig[] | null>(null);
  const [pendingConfigName, setPendingConfigName] = useState("");
  const inputClass =
    "mt-1 w-full rounded-xl border border-slate-200 bg-white px-3 py-2 text-sm outline-none transition focus:border-blue-500";
  const selectClass = `${inputClass} appearance-none pr-10`;
  const isEditing = editingConfigID !== null;
  const submitText = isEditing ? "保存配置" : "添加配置";
  const isPoolForm = form.type === "pool";

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
    setForm((current) => {
      const next = { ...current, ...updates };
      if (updates.type === "pool") {
        next.platform = "custom";
      }
      return next;
    });
    setFormError("");
    if ("platform" in updates || "api_key" in updates || "type" in updates) {
      setCatalog(null);
      setCatalogError("");
    }
  }

  function toFormSMSConfig(config: SMSConfig): SMSConfigForm {
    return {
      id: config.id,
      name: config.name,
      type: config.type,
      platform: config.platform,
      platform_label: config.platform_label || "",
      api_key: config.api_key || "",
      service_id: config.service_id || "dr",
      country_id: config.country_id || 38,
      max_price: String(config.max_price ?? 0),
      max_usage_per_phone: String(config.max_usage_per_phone ?? 1),
      disable_on_error:
        config.disable_on_error === "any_failure" ? "any_failure" : "permanent_only",
    };
  }

  function resetForm(nextConfigs = smsConfigs) {
    setForm(emptySMSConfig(nextConfigs));
    setEditingConfigID(null);
    setFormError("");
  }

  function openCreateForm() {
    setForm(emptySMSConfig(smsConfigs));
    setEditingConfigID(null);
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
    setPendingConfigs(null);
    setPendingConfigName("");
    setMaxUseSyncOpen(false);
    resetForm();
  }

  function validateForm() {
    const normalized = normalizeSMSConfig(form);
    if (!normalized.name) return "请输入配置名称";
    const duplicate = configNames.some((name, index) => {
      if (smsConfigs[index]?.id === editingConfigID) return false;
      return name.toLowerCase() === normalized.name.toLowerCase();
    });
    if (duplicate) return "配置名称不能重复";
    if (normalized.type === "pool") {
      if (!normalized.platform_label?.trim()) return "请输入平台名称";
      if ((normalized.max_usage_per_phone || 0) < 1) return "每个手机号最大使用次数必须大于 0";
      return "";
    }
    if (normalized.platform !== "smsbower" && normalized.platform !== "hero-sms") {
      return "请选择支持的 SMS 平台";
    }
    if (!normalized.api_key?.trim()) return "请输入 API 密钥";
    if (!normalized.service_id?.trim()) return "请选择服务";
    if (!Number.isFinite(Number(form.country_id)) || Number(form.country_id) <= 0) {
      return "请选择国家/地区";
    }
    if (!Number.isFinite(Number(form.max_price)) || Number(form.max_price) < 0) {
      return "最高价格不能小于 0";
    }
    return "";
  }

  function persist(nextConfigs: SMSConfig[], options?: SaveSettingsOptions) {
    const next = { ...settingsDraft, sms_configs: nextConfigs };
    setSettingsDraft(next);
    return saveSettings(next, options);
  }

  async function submitForm() {
    const error = validateForm();
    if (error) {
      setFormError(error);
      return;
    }
    const normalized = normalizeSMSConfig(form);
    const nextConfigs =
      editingConfigID === null
        ? [...smsConfigs, normalized]
        : smsConfigs.map((config) => (config.id === editingConfigID ? { ...config, ...normalized } : config));
    const previous = editingConfigID
      ? smsConfigs.find((config) => config.id === editingConfigID)
      : null;
    const shouldAskMaxUseSync =
      Boolean(previous) &&
      previous?.type === "pool" &&
      normalized.type === "pool" &&
      (previous.max_usage_per_phone || 1) !== (normalized.max_usage_per_phone || 1);
    if (shouldAskMaxUseSync) {
      setPendingConfigs(nextConfigs);
      setPendingConfigName(normalized.name || previous?.name || "当前号池");
      setMaxUseSyncOpen(true);
      return;
    }
    setSaving(true);
    try {
      await persist(nextConfigs);
      setFormOpen(false);
      resetForm(nextConfigs);
    } finally {
      setSaving(false);
    }
  }

  async function submitPendingConfigs(syncPoolMaxUseCount: boolean) {
    if (!pendingConfigs) return;
    setSaving(true);
    try {
      await persist(pendingConfigs, { syncPoolMaxUseCount });
      setFormOpen(false);
      resetForm(pendingConfigs);
      setPendingConfigs(null);
      setPendingConfigName("");
      setMaxUseSyncOpen(false);
    } finally {
      setSaving(false);
    }
  }

  async function removeConfig(id: string) {
    const nextConfigs = smsConfigs.filter((config) => config.id !== id);
    setSaving(true);
    try {
      await persist(nextConfigs);
      if (editingConfigID === id) {
        resetForm(nextConfigs);
      }
    } finally {
      setSaving(false);
    }
  }

  function startEdit(config: SMSConfig) {
    setEditingConfigID(config.id);
    setForm(toFormSMSConfig(config));
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

  async function openImport(config: SMSConfig) {
    setImportConfig(config);
    setImportText("");
    setImportError("");
    setImportOpen(true);
    setPoolLoading(true);
    try {
      const result = await fetchPhonePoolItems(config.id);
      setPoolPreview(result.items || []);
    } catch (error) {
      setPoolPreview([]);
      setImportError(error instanceof Error ? error.message : "加载号池失败");
    } finally {
      setPoolLoading(false);
    }
  }

  function closeImport() {
    if (importing) return;
    setImportOpen(false);
    setImportConfig(null);
    setImportText("");
    setImportError("");
    setPoolPreview([]);
    setSMSPreviewByItemID({});
  }

  async function submitImport() {
    if (!importConfig) return;
    if (!importText.trim()) {
      setImportError("请先输入手机号数据");
      return;
    }
    setImporting(true);
    setImportError("");
    try {
      const result = await importPhonePoolItems(importConfig.id, importText);
      await saveSettings({ ...settingsDraft });
      const refreshed = await fetchPhonePoolItems(importConfig.id);
      setPoolPreview(refreshed.items || []);
      setImportText("");
      if (result.failed > 0) {
        setImportError(result.errors.join("；") || `导入失败 ${result.failed} 条`);
      }
    } catch (error) {
      setImportError(error instanceof Error ? error.message : "导入失败");
    } finally {
      setImporting(false);
    }
  }

  async function removePhonePoolItem(item: PhonePoolItem) {
    if (!importConfig) return;
    setDeletingItemID(item.id);
    setImportError("");
    try {
      await deletePhonePoolItem(item.id);
      await saveSettings({ ...settingsDraft });
      const refreshed = await fetchPhonePoolItems(importConfig.id);
      setPoolPreview(refreshed.items || []);
    } catch (error) {
      setImportError(error instanceof Error ? error.message : "删除手机号失败");
    } finally {
      setDeletingItemID(null);
    }
  }

  async function previewPhonePoolItemSMS(item: PhonePoolItem) {
    setPreviewingItemID(item.id);
    setImportError("");
    try {
      const result = await previewPhonePoolSMS(item.id);
      setSMSPreviewByItemID((current) => ({
        ...current,
        [item.id]: {
          found: Boolean(result.found),
          code: result.code || "",
          previewText: result.preview_text || "",
        },
      }));
    } catch (error) {
      setImportError(error instanceof Error ? error.message : "短信预览失败");
    } finally {
      setPreviewingItemID(null);
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
                description="Codex 授权登录任务需要先保存一条平台短信配置或自定义号池配置。"
              />
            </div>
          )}
          {smsConfigs.map((config) => {
            const summary = config.pool_summary;
            return (
              <div
                key={config.id}
                className="flex h-full flex-col rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4 shadow-sm transition hover:border-blue-200 hover:bg-white/80"
              >
                <div className="flex h-full flex-col gap-4">
                  <div className="min-w-0 flex-1">
                    <div className="mb-3 flex min-w-0 flex-wrap items-center gap-2">
                      <div className="truncate text-lg font-black text-slate-950">
                        {config.name || "-"}
                      </div>
                      <span className="rounded-full bg-emerald-100 px-3 py-1 text-sm font-black text-emerald-700">
                        {platformLabel(config)}
                      </span>
                      <span className="rounded-full bg-slate-100 px-3 py-1 text-sm font-black text-slate-600">
                        {configTypeLabel(config.type)}
                      </span>
                      {editingConfigID === config.id && <Badge status="running" text="编辑中" />}
                    </div>
                    {config.type === "pool" ? (
                      <div className="grid gap-2 sm:grid-cols-2">
                        <ConfigMetric label="可用号码" value={summary?.ready_count || 0} icon={<Database size={15} />} />
                        <ConfigMetric label="剩余总次数" value={summary?.remaining_uses || 0} icon={<FileDigit size={15} />} />
                        <ConfigMetric label="已用尽" value={summary?.used_up_count || 0} />
                        <ConfigMetric label="已禁用" value={summary?.disabled_count || 0} />
                      </div>
                    ) : (
                      <div className="grid gap-2 sm:grid-cols-2">
                        <ConfigMetric label="服务" value={config.service_id || "dr"} />
                        <ConfigMetric label="国家/地区" value={config.country_id || 38} />
                        <ConfigMetric label="最高价格" value={config.max_price || 0} />
                        <ConfigMetric label="API 密钥" value={maskAPIKey(config.api_key || "")} icon={<KeyRound size={15} />} />
                      </div>
                    )}
                  </div>
                  <div className="flex shrink-0 flex-wrap justify-end gap-2">
                    {config.type === "pool" && (
                      <button
                        type="button"
                        onClick={() => openImport(config)}
                        className="inline-flex h-11 items-center gap-2 rounded-xl border border-blue-200 bg-blue-50 px-4 text-sm font-black text-blue-700 shadow-sm"
                      >
                        <Upload size={16} />
                        导入手机号
                      </button>
                    )}
                    <button
                      type="button"
                      onClick={() => startEdit(config)}
                      className="inline-flex h-11 items-center gap-2 rounded-xl border border-slate-200 bg-white px-4 text-sm font-black shadow-sm"
                    >
                      <Pencil size={16} />
                      编辑
                    </button>
                    <button
                      type="button"
                      disabled={busy || saving}
                      onClick={() => removeConfig(config.id)}
                      className="inline-flex h-11 items-center gap-2 rounded-xl border border-rose-200 bg-rose-50 px-4 text-sm font-black text-rose-700 shadow-sm disabled:opacity-50"
                    >
                      <Trash2 size={16} />
                      删除
                    </button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </Card>

      {formOpen && (
        <Modal
          title={isEditing ? "编辑 SMS 配置" : "新增 SMS 配置"}
          subtitle={isPoolForm ? "自定义号池" : "Hero SMS / SMSBower"}
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
              配置类型
              <span className="relative block">
                <select
                  className={selectClass}
                  value={form.type}
                  onChange={(event) =>
                    updateForm({ type: event.target.value as SMSConfigForm["type"] })
                  }
                >
                  {configTypes.map((item) => (
                    <option key={item.value} value={item.value}>
                      {item.label}
                    </option>
                  ))}
                </select>
                <ChevronDown size={16} className="pointer-events-none absolute right-3 top-1/2 mt-0.5 -translate-y-1/2 text-slate-400" />
              </span>
            </label>

            {isPoolForm ? (
              <>
                <label className="text-sm font-bold text-slate-600">
                  平台名称
                  <input
                    className={inputClass}
                    value={form.platform_label}
                    placeholder="boss100 / tgflare"
                    autoComplete="off"
                    onChange={(event) => updateForm({ platform_label: event.target.value })}
                  />
                </label>
                <label className="text-sm font-bold text-slate-600">
                  每号最大使用次数
                  <input
                    className={inputClass}
                    type="number"
                    min={1}
                    value={form.max_usage_per_phone}
                    onChange={(event) => updateForm({ max_usage_per_phone: event.target.value })}
                  />
                </label>
                <label className="text-sm font-bold text-slate-600 md:col-span-2">
                  失败处理策略
                  <span className="relative block">
                    <select
                      className={selectClass}
                      value={form.disable_on_error}
                      onChange={(event) =>
                        updateForm({
                          disable_on_error: event.target.value as SMSConfigForm["disable_on_error"],
                        })
                      }
                    >
                      {disableOnErrorOptions.map((item) => (
                        <option key={item.value} value={item.value}>
                          {item.label}
                        </option>
                      ))}
                    </select>
                    <ChevronDown size={16} className="pointer-events-none absolute right-3 top-1/2 mt-0.5 -translate-y-1/2 text-slate-400" />
                  </span>
                </label>
              </>
            ) : (
              <>
                <label className="text-sm font-bold text-slate-600">
                  短信平台
                  <span className="relative block">
                    <select
                      className={selectClass}
                      value={form.platform || "smsbower"}
                      onChange={(event) => updateForm({ platform: event.target.value })}
                    >
                      {providerPlatforms.map((platform) => (
                        <option key={platform.value} value={platform.value}>
                          {platform.label}
                        </option>
                      ))}
                    </select>
                    <ChevronDown size={16} className="pointer-events-none absolute right-3 top-1/2 mt-0.5 -translate-y-1/2 text-slate-400" />
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
                    <select className={selectClass} value={form.service_id || "dr"} onChange={(event) => updateForm({ service_id: event.target.value })}>
                      {serviceOptions.map((service) => (
                        <option key={service.code} value={service.code}>
                          {serviceLabel(service)}
                        </option>
                      ))}
                    </select>
                    <ChevronDown size={16} className="pointer-events-none absolute right-3 top-1/2 mt-0.5 -translate-y-1/2 text-slate-400" />
                  </span>
                </label>
                <label className="text-sm font-bold text-slate-600">
                  国家/地区
                  <span className="relative block">
                    <select className={selectClass} value={form.country_id} onChange={(event) => updateForm({ country_id: Number(event.target.value) })}>
                      {countryOptions.map((country) => (
                        <option key={country.id} value={country.id}>
                          {countryLabel(country)}
                        </option>
                      ))}
                    </select>
                    <ChevronDown size={16} className="pointer-events-none absolute right-3 top-1/2 mt-0.5 -translate-y-1/2 text-slate-400" />
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
                    placeholder="0"
                    onChange={(event) => updateForm({ max_price: event.target.value })}
                  />
                </label>
              </>
            )}
          </div>
          {formError && (
            <div className="mt-3 rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm font-semibold text-rose-700">
              {formError}
            </div>
          )}
          <div className="mt-4 flex flex-wrap justify-end gap-2">
            <button type="button" onClick={closeForm} disabled={saving} className="inline-flex items-center gap-2 rounded-xl border bg-white px-3 py-2 text-sm font-bold disabled:opacity-50">
              <X size={16} />
              取消
            </button>
            <button type="button" onClick={submitForm} disabled={busy || saving} className="inline-flex items-center gap-2 rounded-xl bg-slate-950 px-3 py-2 text-sm font-bold text-white disabled:opacity-50">
              {isEditing ? <Save size={16} /> : <Plus size={16} />}
              {busy || saving ? "保存中..." : submitText}
            </button>
          </div>
        </Modal>
      )}

      {importOpen && importConfig && (
        <Modal
          title={`导入手机号 · ${importConfig.name}`}
          subtitle={`${platformLabel(importConfig)} · 最大使用次数 ${importConfig.max_usage_per_phone || 1}`}
          onClose={closeImport}
        >
          <div className="grid gap-3">
            <div className="rounded-2xl border border-blue-100 bg-blue-50/80 px-4 py-3 text-sm font-semibold text-blue-900">
              一行一个，支持 `手机号----链接`、`手机号|链接`、`手机号:https://...`。手机号会统一存成带 `+` 的格式。
            </div>
            <label className="text-sm font-bold text-slate-600">
              批量导入
              <textarea
                className={`${inputClass} min-h-[12rem] resize-y font-mono`}
                value={importText}
                placeholder={"+18352622848----https://example.com/api/record?token=...\n13808954028|http://boss100.fit/api/msgForeign?code=..."}
                onChange={(event) => setImportText(event.target.value)}
              />
            </label>
            {importError && (
              <div className="rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm font-semibold text-rose-700">
                {importError}
              </div>
            )}
            <div className="rounded-2xl border border-slate-200/80 bg-slate-50/70 p-4">
              <div className="mb-3 flex items-center justify-between gap-2">
                <div className="text-sm font-black text-slate-800">当前号池预览</div>
                <div className="text-xs font-semibold text-slate-500">
                  {poolLoading ? "加载中..." : `已加载 ${poolPreview.length} 条`}
                </div>
              </div>
              <div className="max-h-64 overflow-auto rounded-xl border border-slate-200 bg-white">
                {poolPreview.length === 0 ? (
                  <div className="px-4 py-6 text-sm text-slate-500">暂无手机号数据</div>
                ) : (
                  <table className="min-w-full text-sm">
                    <thead className="bg-slate-50 text-left text-slate-500">
                      <tr>
                        <th className="px-3 py-2">手机号</th>
                        <th className="px-3 py-2">状态</th>
                        <th className="px-3 py-2">使用次数</th>
                        <th className="px-3 py-2">操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {poolPreview.slice(0, 20).map((item) => {
                        const preview = smsPreviewByItemID[item.id];
                        return (
                          <Fragment key={item.id}>
                            <tr className="border-t border-slate-100">
                              <td className="px-3 py-2 font-mono text-slate-700">{item.phone_number}</td>
                              <td className="px-3 py-2 text-slate-600">
                                <Badge {...phonePoolStatusBadge(item.status)} />
                              </td>
                              <td className="px-3 py-2 text-slate-600">{item.use_count}/{item.max_use_count}</td>
                              <td className="px-3 py-2">
                                <div className="flex flex-wrap gap-2">
                                  <button
                                    type="button"
                                    onClick={() => previewPhonePoolItemSMS(item)}
                                    disabled={previewingItemID === item.id}
                                    className="inline-flex items-center gap-1 rounded-lg border border-blue-200 bg-blue-50 px-2 py-1 text-xs font-bold text-blue-700 disabled:opacity-50"
                                  >
                                    {previewingItemID === item.id ? "预览中..." : "短信预览"}
                                  </button>
                                  <button
                                    type="button"
                                    onClick={() => removePhonePoolItem(item)}
                                    disabled={deletingItemID === item.id || item.status === "reserved"}
                                    className="inline-flex items-center gap-1 rounded-lg border border-rose-200 bg-rose-50 px-2 py-1 text-xs font-bold text-rose-700 disabled:opacity-50"
                                    title={item.status === "reserved" ? "使用中的手机号不能删除" : "删除该手机号"}
                                  >
                                    <Trash2 size={12} />
                                    {item.status === "reserved"
                                      ? "使用中"
                                      : deletingItemID === item.id
                                        ? "删除中..."
                                        : "删除"}
                                  </button>
                                </div>
                              </td>
                            </tr>
                            {preview && (
                              <tr className="border-t border-slate-100 bg-slate-50/70">
                                <td colSpan={4} className="px-3 py-3">
                                  <div className="grid gap-2 text-sm">
                                    <div className="flex flex-wrap items-center gap-2">
                                      <span className="font-bold text-slate-700">短信预览结果</span>
                                      <Badge status={preview.found ? "success" : "running"} text={preview.found ? `验证码 ${preview.code}` : "未提取到验证码"} />
                                    </div>
                                    <pre className="overflow-auto rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs text-slate-600 whitespace-pre-wrap break-all">
                                      {preview.previewText || "接口返回为空"}
                                    </pre>
                                  </div>
                                </td>
                              </tr>
                            )}
                          </Fragment>
                        );
                      })}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
          </div>
          <div className="mt-4 flex flex-wrap justify-end gap-2">
            <button type="button" onClick={closeImport} disabled={importing} className="inline-flex items-center gap-2 rounded-xl border bg-white px-3 py-2 text-sm font-bold disabled:opacity-50">
              <X size={16} />
              关闭
            </button>
            <button type="button" onClick={submitImport} disabled={importing || !importText.trim()} className="inline-flex items-center gap-2 rounded-xl bg-slate-950 px-3 py-2 text-sm font-bold text-white disabled:opacity-50">
              <Upload size={16} />
              {importing ? "导入中..." : "开始导入"}
            </button>
          </div>
        </Modal>
      )}

      {maxUseSyncOpen && pendingConfigs && (
        <Modal
          title="同步最大使用次数"
          subtitle={pendingConfigName}
          onClose={() => {
            if (saving) return;
            setMaxUseSyncOpen(false);
            setPendingConfigs(null);
            setPendingConfigName("");
          }}
        >
          <div className="grid gap-3">
            <div className="rounded-2xl border border-amber-100 bg-amber-50/80 px-4 py-3 text-sm font-semibold text-amber-900">
              你修改了该号池配置的“每号最大使用次数”。请选择是否把已经导入的手机号也同步更新为当前次数。
            </div>
            <div className="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600">
              选择“仅新增号码使用新次数”后，已有号码保持当前 `max_use_count` 不变，之后新导入的号码会使用新的默认次数。
            </div>
          </div>
          <div className="mt-4 flex flex-wrap justify-end gap-2">
            <button
              type="button"
              onClick={() => submitPendingConfigs(true)}
              disabled={saving}
              className="inline-flex items-center gap-2 rounded-xl border bg-white px-3 py-2 text-sm font-bold disabled:opacity-50"
            >
              同步已有号码
            </button>
            <button
              type="button"
              onClick={() => submitPendingConfigs(false)}
              disabled={saving}
              className="inline-flex items-center gap-2 rounded-xl bg-slate-950 px-3 py-2 text-sm font-bold text-white disabled:opacity-50"
            >
              仅新增号码使用新次数
            </button>
          </div>
        </Modal>
      )}
    </>
  );
}
