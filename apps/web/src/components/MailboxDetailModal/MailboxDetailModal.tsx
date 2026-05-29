import { formatToken, jobTypeText, resultText } from "../../lib/format";
import type { Mailbox, MailboxUpdate } from "../../types";
import { Field } from "../Field/Field";
import { InfoRow } from "../InfoRow/InfoRow";
import { Modal } from "../Modal/Modal";
import styles from "./MailboxDetailModal.module.css";

export function MailboxDetailModal({
  detail,
  detailDraft,
  credentialsOpen,
  busy,
  onClose,
  onToggleCredentials,
  onUpdateDraft,
  onUpdateCredentialLine,
  onSave,
}: {
  detail: Mailbox;
  detailDraft: MailboxUpdate;
  credentialsOpen: boolean;
  busy: boolean;
  onClose: () => void;
  onToggleCredentials: () => void;
  onUpdateDraft: (key: keyof MailboxUpdate, value: string) => void;
  onUpdateCredentialLine: (value: string) => void;
  onSave: () => void;
}) {
  return (
    <Modal
      title="邮箱详情"
      subtitle={detailDraft.email || detail.email}
      onClose={onClose}
      footer={
        <div className="flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            className="rounded-xl border bg-white px-3 py-2 font-bold"
          >
            取消
          </button>
          <button
            type="button"
            onClick={onSave}
            disabled={busy || !detailDraft.email.trim()}
            className="rounded-xl bg-slate-950 px-3 py-2 font-bold text-white disabled:opacity-50"
          >
            保存
          </button>
        </div>
      }
    >
      <div className="space-y-3 text-sm">
        <Field label="OpenAI 密码">
          <input
            value={detailDraft.register_password || ""}
            onChange={(event) =>
              onUpdateDraft("register_password", event.target.value)
            }
            className="w-full rounded-xl border bg-white px-3 py-2 font-mono text-sm outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="OpenAI 登录密码"
          />
        </Field>
        <InfoRow
          label="最近任务"
          value={
            detail.last_job_id
              ? `#${detail.last_job_id} ${jobTypeText(detail.last_job_type)} ${resultText(detail.last_job_status)}`
              : "-"
          }
        />
        <InfoRow label="绑定手机号" value={detail.phone_number || "-"} />
        <InfoRow
          label="失败信息"
          value={detail.last_job_error || detail.last_error || "-"}
        />
        <div>
          <div className="mb-2 font-bold text-slate-600">Token</div>
          <pre className="max-h-56 overflow-auto rounded-xl border bg-slate-50 p-3 text-xs">
            {formatToken(detail.token_json)}
          </pre>
        </div>
        <div className="rounded-xl border bg-slate-50 p-3">
          <button
            type="button"
            onClick={onToggleCredentials}
            className="flex w-full items-center justify-between gap-3 text-left font-bold text-slate-700"
          >
            <span>邮箱凭据</span>
            <span className="text-xs text-slate-500">
              {credentialsOpen ? "收起" : "展开"}
            </span>
          </button>
          {credentialsOpen && (
            <textarea
              value={[
                detailDraft.email || "",
                detailDraft.password || "",
                detailDraft.client_id || "",
                detailDraft.access_token || "",
              ].join("----")}
              onChange={(event) => onUpdateCredentialLine(event.target.value)}
              className="mt-2 h-24 w-full rounded-xl border bg-white p-3 font-mono text-xs outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="email@example.com----password----client_id----refresh_token"
            />
          )}
        </div>
      </div>
    </Modal>
  );
}
