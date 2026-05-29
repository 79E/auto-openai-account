import { Link } from "react-router-dom";
import { MessageSquareText, PlugZap } from "lucide-react";
import { Badge } from "../../components/Badge/Badge";
import { Card } from "../../components/Card/Card";

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
    {
      name: "sms-provider",
      status: "已接入",
      desc: "负责 Hero SMS 和 SMSBower 手机号获取与短信验证码轮询。",
      action: (
        <Link
          to="/sms"
          className="inline-flex items-center gap-2 rounded-xl border bg-white px-3 py-2 text-sm font-bold"
        >
          <MessageSquareText size={16} />
          SMS 配置
        </Link>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <Card title="插件列表" icon={<PlugZap size={18} />}>
        <p className="text-sm text-slate-500">当前已接入的功能插件。</p>
      </Card>
      <div className="grid gap-3 md:grid-cols-2">
        {plugins.map((p) => (
          <Card key={p.name} title={p.name} icon={<PlugZap size={18} />} actions={p.action}>
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
