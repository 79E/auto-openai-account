import { defaultPassword } from "./constants";
import type { SettingsPayload } from "../types";

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
    fixed_password: settings.fixed_password || defaultPassword,
  };
}

