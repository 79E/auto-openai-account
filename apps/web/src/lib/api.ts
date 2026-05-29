import { defaultPassword } from "./constants";
import type { SMSCatalog, SettingsPayload } from "../types";

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
          platform: config.platform || "smsbower",
          api_key: config.api_key || "",
          service_id: config.service_id || "dr",
          country_id: Number(config.country_id) || 38,
          max_price: Number(config.max_price) || 0,
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
