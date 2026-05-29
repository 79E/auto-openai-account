import { defaultPassword } from "./constants";
import type { SMSCatalog, SettingsPayload } from "../types";

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

export function normalizeSettingsPayload(settings: SettingsPayload): SettingsPayload {
  return {
    ...settings,
    proxies: Array.isArray(settings.proxies) ? settings.proxies : [],
    sms_configs: Array.isArray(settings.sms_configs)
      ? settings.sms_configs.map((config) => ({
          name: config.name || "",
          platform: config.platform || "smsbower",
          api_key: config.api_key || "",
          service_id: config.service_id || "dr",
          country_id: Number(config.country_id) || 38,
          max_price: Number(config.max_price) || 0,
        }))
      : [],
    fixed_password: settings.fixed_password || defaultPassword,
  };
}

export function fetchSMSCatalog(platform: string, apiKey: string) {
  return api<SMSCatalog>("/api/sms/catalog", {
    method: "POST",
    body: JSON.stringify({ platform, api_key: apiKey }),
  });
}
