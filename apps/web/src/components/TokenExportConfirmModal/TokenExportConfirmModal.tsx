import type { TokenExportConfirm } from "../../types";
import { Modal } from "../Modal/Modal";
import styles from "./TokenExportConfirmModal.module.css";

export function TokenExportConfirmModal({
  exportInfo,
  onClose,
  onConfirm,
}: {
  exportInfo: NonNullable<TokenExportConfirm>;
  onClose: () => void;
  onConfirm: () => void;
}) {
  return (
    <Modal
      title="确认导出 Token"
      subtitle={`任务 #${exportInfo.jobId}`}
      onClose={onClose}
    >
      <div className="space-y-4 text-sm">
        <div className="rounded-2xl border border-blue-100 bg-blue-50 p-4 text-slate-700">
          <div className="text-base font-black text-slate-950">
            可导出 {exportInfo.count} 条数据
          </div>
          <div className="mt-2 leading-6">
            本次只会导出该任务中执行成功，并且已经生成 token 的邮箱数据。
          </div>
        </div>
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
            onClick={onConfirm}
            className="rounded-xl bg-slate-950 px-3 py-2 font-bold text-white"
          >
            确认导出
          </button>
        </div>
      </div>
    </Modal>
  );
}
