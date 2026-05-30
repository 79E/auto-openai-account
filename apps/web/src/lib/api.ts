import { defaultPassword } from "./constants";
import type { PhonePoolItem, SMSCatalog, SettingsPayload } from "../types";

type RawSettingsPayload = Partial<SettingsPayload> & {
  proxy_mode?: string;
  proxies?: string[];
};

export function createSettingsItemID() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID().replace(/-/g, "");
  }
  return `${Date.now().toString(16)}${Math.random().toString(16).slice(2)}`;
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
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

export function normalizeSettingsPayload(settings: RawSettingsPayload): SettingsPayload {
  const proxies = Array.isArray(settings.proxies)
    ? settings.proxies.filter(Boolean)
    : [];
  const proxyGroups = Array.isArray(settings.proxy_groups)
    ? settings.proxy_groups
        .map((group) => ({
          id: group?.id || createSettingsItemID(),
          name: group?.name || "",
          mode: group?.mode === "round_robin" ? "round_robin" : "random",
          proxies: Array.isArray(group?.proxies)
            ? group.proxies.filter(Boolean)
            : [],
        }))
        .filter((group) => group.name.trim() && group.proxies.length > 0)
    : [];
  return {
    proxy_groups:
      proxyGroups.length > 0
        ? proxyGroups
        : proxies.length > 0
          ? [{ id: createSettingsItemID(), name: "默认分组", mode: settings.proxy_mode === "round_robin" ? "round_robin" : "random", proxies }]
          : [],
    register_concurrency: Number(settings.register_concurrency) || 1,
    password_mode: settings.password_mode || "random",
    fixed_password: settings.fixed_password || defaultPassword,
    imap_host: settings.imap_host || "outlook.office365.com",
    imap_port: Number(settings.imap_port) || 993,
    imap_auth_mode: settings.imap_auth_mode || "auto",
    otp_timeout_seconds: Number(settings.otp_timeout_seconds) || 180,
    otp_poll_interval_seconds: Number(settings.otp_poll_interval_seconds) || 5,
    listen: settings.listen || ":8080",
    sms_configs: Array.isArray(settings.sms_configs)
      ? settings.sms_configs.map((config) => ({
          id: config.id || createSettingsItemID(),
          name: config.name || "",
          type: config.type === "pool" ? "pool" : "provider",
          platform:
            config.type === "pool"
              ? config.platform || "custom"
              : config.platform || "smsbower",
          platform_label: config.platform_label || "",
          api_key: config.api_key || "",
          service_id: config.service_id || "dr",
          country_id: Number(config.country_id) || 38,
          max_price: Number(config.max_price) || 0,
          max_usage_per_phone:
            Number(config.max_usage_per_phone) > 0
              ? Number(config.max_usage_per_phone)
              : 1,
          disable_on_error:
            config.disable_on_error === "any_failure"
              ? "any_failure"
              : "permanent_only",
          pool_summary: config.pool_summary
            ? {
                total_count: Number(config.pool_summary.total_count) || 0,
                ready_count: Number(config.pool_summary.ready_count) || 0,
                reserved_count: Number(config.pool_summary.reserved_count) || 0,
                used_up_count: Number(config.pool_summary.used_up_count) || 0,
                disabled_count: Number(config.pool_summary.disabled_count) || 0,
                error_count: Number(config.pool_summary.error_count) || 0,
                remaining_uses:
                  Number(config.pool_summary.remaining_uses) || 0,
              }
            : undefined,
        }))
      : [],
  };
}

export function fetchSMSCatalog(platform: string, apiKey: string) {
  return api<SMSCatalog>("/api/sms/catalog", {
    method: "POST",
    body: JSON.stringify({ platform, api_key: apiKey }),
  });
}

export function importPhonePoolItems(configID: string, text: string) {
  return api<{ imported: number; skipped: number; failed: number; errors: string[] }>(
    `/api/sms-configs/${configID}/phone-pool/import`,
    {
      method: "POST",
      body: JSON.stringify({ text }),
    },
  );
}

export function fetchPhonePoolItems(configID: string, status = "") {
  const search = status ? `?status=${encodeURIComponent(status)}` : "";
  return api<{ total: number; items: PhonePoolItem[] }>(
    `/api/sms-configs/${configID}/phone-pool${search}`,
  );
}

export function deletePhonePoolItem(id: number) {
  return api<{ ok: boolean }>(`/api/phone-pool-items/${id}`, {
    method: "DELETE",
  });
}

export function previewPhonePoolSMS(id: number) {
  return api<{ found: boolean; code: string; preview_text: string }>(
    `/api/phone-pool-preview/${id}`,
    {
      method: "POST",
    },
  );
}
