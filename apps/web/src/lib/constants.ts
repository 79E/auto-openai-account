import type { Stats } from "../types";

export const emptyStats: Stats = { mailboxes: {}, jobs: {} };
export const appName = "Auto OpenAI Account";
export const defaultPassword = "Mima1234567890.";
export const nav: Array<{ path: string; label: string; end?: boolean }> = [
  { path: "/", label: "总览", end: true },
  { path: "/mailboxes", label: "邮箱池" },
  { path: "/jobs", label: "任务" },
  { path: "/proxies", label: "代理池" },
  { path: "/plugins", label: "插件" },
];
export const routeTitles: Record<string, string> = {
  "/": "总览",
  "/mailboxes": "邮箱池",
  "/jobs": "任务",
  "/proxies": "代理池",
  "/plugins": "插件",
};

