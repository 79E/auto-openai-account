import { PlugZap } from "lucide-react";
import { Badge } from "../../components/Badge/Badge";
import { Card } from "../../components/Card/Card";
import styles from "./PluginsPage.module.css";

export function PluginsPage() {
  const plugins = [
    {
      name: "openai-provider",
      status: "已接入",
      desc: "负责 OpenAI 注册、登录和 token 换取流程。",
    },
    {
      name: "mail-outlook",
      status: "已接入",
      desc: "负责登录 Outlook 邮箱并读取邮件。",
    },
    {
      name: "openai-otp",
      status: "已接入",
      desc: "通用验证码解析插件，负责从邮件 HTML 或正文内容中提取 OpenAI 验证码。",
    },
    {
      name: "proxy-pool",
      status: "已实现",
      desc: "负责代理池、测速和任务代理选择。",
    },
  ];
  return (
    <div className="space-y-4">
      <Card title="插件列表" icon={<PlugZap size={18} />}>
        <p className="text-sm text-slate-500">当前已接入的功能插件。</p>
      </Card>
      <div className="grid gap-3 md:grid-cols-2">
        {plugins.map((p) => (
          <Card key={p.name} title={p.name} icon={<PlugZap size={18} />}>
            <div className="mb-3">
              <Badge status="success" text={p.status} />
            </div>
            <p className="text-sm leading-6 text-slate-500">{p.desc}</p>
          </Card>
        ))}
      </div>
    </div>
  );
}
