import { Card, CardHeader } from '../components/ui/Card';

export function SettingsPage() {
  return (
    <div className="space-y-6">
      <div><p className="text-sm text-blue-300">Settings</p><h1 className="mt-2 text-3xl font-semibold text-white">设置</h1></div>
      <Card>
        <CardHeader title="第一版说明" description="当前设置页只做只读说明，在线修改配置放到后续阶段。" />
        <div className="space-y-3 text-sm text-slate-400">
          <p>API 默认地址：<code className="text-slate-200">127.0.0.1:8080</code></p>
          <p>HTTP 代理默认地址：<code className="text-slate-200">127.0.0.1:1081</code></p>
          <p>SOCKS5 默认地址：<code className="text-slate-200">127.0.0.1:1080</code></p>
          <p>配置文件：<code className="text-slate-200">config.yaml</code></p>
        </div>
      </Card>
    </div>
  );
}
