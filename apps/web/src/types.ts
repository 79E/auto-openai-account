export type MailboxView = "all" | "unused" | "used" | "registered" | "abnormal";

export type Stats = {
  mailboxes: Record<string, number>;
  jobs: Record<string, number>;
};

export type Mailbox = {
  id: number;
  email: string;
  password?: string;
  client_id?: string;
  access_token?: string;
  register_password?: string;
  token_json?: string;
  status: string;
  status_text: string;
  current_step?: string;
  current_step_index?: number;
  current_step_total?: number;
  proxy?: string;
  phone_number?: string;
  last_error?: string;
  last_job_id?: number;
  last_job_type?: string;
  last_job_status?: string;
  last_job_error?: string;
};

export type MailboxUpdate = Pick<
  Mailbox,
  "email" | "password" | "client_id" | "access_token" | "register_password"
>;

export type Job = {
  id: number;
  type: string;
  status: string;
  requested_count: number;
  total_count: number;
  success_count: number;
  failed_count: number;
  success_rate: number;
  items?: JobItem[];
};

export type JobItem = {
  id: number;
  email: string;
  status: string;
  error?: string;
  duration_ms: number;
};

export type JobTokenExportItem = {
  [key: string]: unknown;
};

export type RuntimeLog = {
  id: number;
  email: string;
  level: string;
  step: string;
  step_index: number;
  step_total: number;
  message: string;
  created_at: string;
};

export type SMSConfig = {
  name: string;
  platform: string;
  api_key: string;
  service_id: string;
  country_id: number;
  max_price: number;
};

export type SMSCatalogService = {
  code: string;
  name: string;
};

export type SMSCatalogCountry = {
  id: number;
  rus?: string;
  eng?: string;
  chn?: string;
  visible?: number;
  retry?: number;
};

export type SMSCatalog = {
  services: SMSCatalogService[];
  countries: SMSCatalogCountry[];
};

export type ProxyGroup = {
  name: string;
  mode: string;
  proxies: string[];
};

export type SettingsPayload = {
  proxy_groups: ProxyGroup[];
  register_concurrency: number;
  password_mode: string;
  fixed_password: string;
  imap_host: string;
  imap_port: number;
  imap_auth_mode: string;
  otp_timeout_seconds: number;
  otp_poll_interval_seconds: number;
  listen: string;
  sms_configs: SMSConfig[];
};

export type ProxyTestResult = {
  proxy: string;
  ok: boolean;
  ip?: string;
  latency_ms: number;
  error?: string;
};

export type ToastState = {
  message: string;
  type: "success" | "error" | "info";
} | null;

export type TokenExportConfirm = {
  jobId: number;
  count: number;
  items: JobTokenExportItem[];
} | null;
