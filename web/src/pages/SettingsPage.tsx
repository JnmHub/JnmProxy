import { KeyRound, Save, Trash2 } from 'lucide-react';
import { useState } from 'react';
import { getAdminToken, saveAdminToken } from '../api/client';
import { Button } from '../components/ui/Button';
import { Card, CardHeader } from '../components/ui/Card';
import { Field, Input } from '../components/ui/Input';

export function SettingsPage() {
  const [token, setToken] = useState(getAdminToken());
  const [saved, setSaved] = useState(false);

  const save = () => {
    saveAdminToken(token);
    setSaved(true);
  };
  const clear = () => {
    setToken('');
    saveAdminToken('');
    setSaved(true);
  };

  return (
    <div className="space-y-6">
      <div>
        <p className="text-sm text-blue-300">Settings</p>
        <h1 className="mt-2 text-3xl font-semibold text-white">设置</h1>
        <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-400">配置前端访问管理 API 所需的本地管理令牌。代理 HTTP/SOCKS5 凭证不受这里影响。</p>
      </div>

      <Card>
        <CardHeader title="管理 API Token" description="后端 admin.token 不为空时，前端请求会自动携带 Authorization: Bearer <token>。" />
        <div className="grid gap-4">
          <Field label="Token">
            <div className="relative">
              <KeyRound className="absolute left-3 top-2.5 h-4 w-4 text-slate-500" />
              <Input className="pl-9" type="password" value={token} onChange={(event) => { setToken(event.target.value); setSaved(false); }} placeholder="填写 config.yaml 里的 admin.token" />
            </div>
          </Field>
          <div className="flex flex-wrap gap-2">
            <Button variant="primary" onClick={save}><Save className="h-4 w-4" />保存到浏览器</Button>
            <Button variant="danger" onClick={clear}><Trash2 className="h-4 w-4" />清除</Button>
          </div>
          {saved ? <div className="rounded-2xl border border-emerald-400/30 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-200">已更新本浏览器保存的管理令牌。</div> : null}
        </div>
      </Card>

      <Card>
        <CardHeader title="默认地址" description="这些地址来自默认配置，可在 config.yaml 中调整。" />
        <div className="space-y-3 text-sm text-slate-400">
          <p>管理后台/API：<code className="text-slate-200">127.0.0.1:8080</code></p>
          <p>HTTP 代理默认地址：<code className="text-slate-200">127.0.0.1:1081</code></p>
          <p>SOCKS5 默认地址：<code className="text-slate-200">127.0.0.1:1080</code></p>
          <p>配置文件：<code className="text-slate-200">config.yaml</code></p>
        </div>
      </Card>
    </div>
  );
}
