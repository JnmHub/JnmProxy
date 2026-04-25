import { useQuery } from '@tanstack/react-query';
import { AlertTriangle, CheckCircle2, PlugZap, ShieldCheck, Terminal, XCircle, Zap } from 'lucide-react';
import type { ReactNode } from 'react';
import { getSingBoxStatus, getSystemHealth } from '../api/system';
import type { SingBoxStatus } from '../api/types';
import { Badge } from '../components/ui/Badge';
import { Card, CardHeader } from '../components/ui/Card';
import { LoadingState } from '../components/ui/LoadingState';
import { DataTable } from '../components/ui/Table';
import { formatTime } from '../utils/format';
import { cx } from '../utils/status';

const recommendedCommand = 'go run -tags "with_quic with_utls" ./cmd/jnmproxy';

export function SystemPage() {
  const healthQuery = useQuery({ queryKey: ['system', 'health'], queryFn: getSystemHealth });
  const singBoxQuery = useQuery({ queryKey: ['system', 'sing-box'], queryFn: getSingBoxStatus });
  const singBox = singBoxQuery.data;
  const protocols = singBox?.supported_protocols ?? [];
  const apiHealthy = healthQuery.data?.status === 'ok';
  const adapterReady = Boolean(singBox?.adapter_configured);
  const singBoxReady = Boolean(singBox?.enabled && adapterReady);

  return (
    <div className="space-y-6">
      <div>
        <p className="text-sm font-medium text-blue-300">System Diagnostics</p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight text-white">系统诊断</h1>
        <p className="mt-2 max-w-4xl text-sm leading-6 text-slate-400">
          这里专门用来排查“节点看起来可用，但代理请求失败”的问题，重点展示构建标签、sing-box、协议能力和健康检查目标。
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <StatusTile
          title="API 服务"
          value={apiHealthy ? '正常' : healthQuery.isError ? '异常' : '检查中'}
          description={healthQuery.data?.time ? `后端时间：${formatTime(healthQuery.data.time)}` : '来自 /api/v1/system/health'}
          tone={apiHealthy ? 'success' : healthQuery.isError ? 'danger' : 'warning'}
          icon={apiHealthy ? <CheckCircle2 className="h-5 w-5" /> : <XCircle className="h-5 w-5" />}
        />
        <StatusTile
          title="sing-box"
          value={singBoxReady ? '已就绪' : singBox?.enabled ? '未接入节点适配' : '未启用'}
          description={singBox?.version ? `依赖版本：${singBox.version}` : '复杂协议出站适配状态'}
          tone={singBoxReady ? 'success' : singBox?.enabled ? 'warning' : 'muted'}
          icon={<PlugZap className="h-5 w-5" />}
        />
        <StatusTile
          title="QUIC 构建"
          value={singBox?.quic_enabled ? '已启用' : '未启用'}
          description="Hysteria2 / TUIC 需要 with_quic"
          tone={singBox?.quic_enabled ? 'success' : 'warning'}
          icon={<Zap className="h-5 w-5" />}
        />
        <StatusTile
          title="uTLS 构建"
          value={singBox?.utls_enabled ? '已启用' : '未启用'}
          description="REALITY / fingerprint 节点需要 with_utls"
          tone={singBox?.utls_enabled ? 'success' : 'warning'}
          icon={<ShieldCheck className="h-5 w-5" />}
        />
      </div>

      <div className="grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
        <Card>
          <CardHeader title="运行能力总览" description="这些字段全部来自后端状态接口，不需要前端猜测。" />
          {singBoxQuery.isLoading ? <LoadingState title="正在读取运行能力" /> : (
            <div className="grid gap-4 lg:grid-cols-2">
              {buildCapabilities(singBox).map((item) => (
                <CapabilityCard key={item.title} {...item} />
              ))}
            </div>
          )}
        </Card>

        <Card>
          <CardHeader title="推荐启动方式" description="机场节点里有 HY2、TUIC、REALITY 时优先用这个命令。" />
          <CommandBlock command={recommendedCommand} />
          <div className="mt-4 space-y-3 text-sm">
            <InfoRow label="健康检查目标"><span className="font-mono text-xs text-blue-200">{singBox?.health_check_target ?? '—'}</span></InfoRow>
            <InfoRow label="运行模式"><span className="font-mono text-slate-300">{singBox?.mode ?? '—'}</span></InfoRow>
            <InfoRow label="配置版本"><span className="font-mono text-slate-300">{singBox?.config_version ?? '—'}</span></InfoRow>
            <InfoRow label="许可证"><span className="text-right text-slate-300">{singBox?.license ?? '—'}</span></InfoRow>
          </div>
        </Card>
      </div>

      <div className="grid gap-4 xl:grid-cols-3">
        <Card className="xl:col-span-2">
          <CardHeader title="支持协议" description="如果协议不在列表里，对应节点不会进入 sing-box 支持状态。" />
          {singBoxQuery.isLoading ? <LoadingState title="正在加载协议列表" /> : (
            <DataTable columns={['协议', '说明']} empty={!protocols.length}>
              {protocols.map((protocol) => (
                <tr key={protocol}>
                  <td className="px-4 py-3 font-mono text-blue-200">{protocol}</td>
                  <td className="px-4 py-3 text-slate-400">{protocolHint(protocol, singBox)}</td>
                </tr>
              ))}
            </DataTable>
          )}
        </Card>

        <Card>
          <CardHeader title="节点进入随机池条件" description="节点不是只看 alive，还要满足运行时可转发。" />
          <div className="space-y-3">
            <ChecklistItem ok label="节点本身已启用" />
            <ChecklistItem ok label="adapter 或 sing-box 状态为支持" />
            <ChecklistItem ok label="alive_status 不是 dead" />
            <ChecklistItem ok label="没有处于内存失败熔断期" />
            <ChecklistItem ok label="凭证绑定范围能匹配到它" />
          </div>
          <p className="mt-4 rounded-2xl border border-blue-400/20 bg-blue-500/10 px-4 py-3 text-xs leading-5 text-blue-100">
            随机策略现在使用洗牌袋算法：同一凭证的一轮候选节点会尽量都用一次，再重新洗牌。
          </p>
        </Card>
      </div>

      <Card>
        <CardHeader title="常见错误怎么判断" description="按现象快速定位，不需要先去翻日志。" />
        <div className="grid gap-4 lg:grid-cols-3">
          <TroubleCard
            title="SOCKS5 返回错误 5"
            symptom="curl 提示 Can't complete SOCKS5 connection"
            action="优先看节点是否需要 with_utls、with_quic，或健康检查是否把节点标成 dead。"
          />
          <TroubleCard
            title="HY2 / TUIC 不支持"
            symptom="节点 adapter 显示 error 或 unsupported"
            action="使用 with_quic 重新启动；没有 QUIC 构建时这些 UDP/QUIC 协议不会进入支持状态。"
          />
          <TroubleCard
            title="REALITY 节点失败"
            symptom="VLESS REALITY 或 fingerprint 节点不可用"
            action="使用 with_utls 重新启动，并确认订阅解析出的 server_name、fingerprint、public_key 正确。"
          />
        </div>
      </Card>
    </div>
  );
}

function buildCapabilities(singBox?: SingBoxStatus): Array<{
  title: string;
  enabled: boolean;
  enabledText: string;
  disabledText: string;
  description: string;
}> {
  return [
    {
      title: 'sing-box 出站',
      enabled: Boolean(singBox?.enabled),
      enabledText: '已启用',
      disabledText: '未启用',
      description: '复杂协议通过内嵌 sing-box 依赖连接上游节点。',
    },
    {
      title: '节点 Adapter',
      enabled: Boolean(singBox?.adapter_configured),
      enabledText: '已接入',
      disabledText: '未接入',
      description: '用于把节点配置转换为运行时可拨号的出站适配器。',
    },
    {
      title: 'QUIC 能力',
      enabled: Boolean(singBox?.quic_enabled),
      enabledText: '已包含 with_quic',
      disabledText: '缺少 with_quic',
      description: 'Hysteria2、TUIC 依赖这个能力。',
    },
    {
      title: 'uTLS 能力',
      enabled: Boolean(singBox?.utls_enabled),
      enabledText: '已包含 with_utls',
      disabledText: '缺少 with_utls',
      description: 'REALITY、fingerprint、部分 TLS 伪装依赖这个能力。',
    },
    {
      title: 'UDP 出站',
      enabled: Boolean(singBox?.enable_udp),
      enabledText: '已启用',
      disabledText: '未启用',
      description: '是否允许 sing-box 出站配置携带 UDP 能力。',
    },
    {
      title: '原生 HTTP/SOCKS',
      enabled: Boolean(singBox?.prefer_native_http_socks),
      enabledText: '优先原生',
      disabledText: '不优先原生',
      description: '简单 HTTP/SOCKS 节点可由 JnmProxy 原生拨号，减少额外适配成本。',
    },
  ];
}

function protocolHint(protocol: string, singBox?: SingBoxStatus) {
  const normalized = protocol.toLowerCase();
  if (['hysteria2', 'hy2', 'tuic'].includes(normalized)) return singBox?.quic_enabled ? 'QUIC 协议，当前构建可用。' : '需要 with_quic 构建。';
  if (['vless', 'trojan', 'vmess'].includes(normalized)) return '复杂 TCP/TLS 节点，部分 REALITY/fingerprint 场景需要 with_utls。';
  if (['http', 'https', 'socks', 'socks5', 'socks5h'].includes(normalized)) return '简单代理协议，可走原生或 sing-box 出站。';
  if (['ss', 'shadowsocks'].includes(normalized)) return 'Shadowsocks 节点，当前已进入支持协议列表。';
  return '由后端 sing-box 状态接口返回的支持协议。';
}

function StatusTile({ title, value, description, tone, icon }: { title: string; value: string; description: string; tone: 'success' | 'warning' | 'danger' | 'muted'; icon: ReactNode }) {
  const tones = {
    success: 'border-emerald-400/20 bg-emerald-500/10 text-emerald-200',
    warning: 'border-amber-400/20 bg-amber-500/10 text-amber-200',
    danger: 'border-red-400/20 bg-red-500/10 text-red-200',
    muted: 'border-slate-700 bg-slate-900/70 text-slate-300',
  };
  return (
    <Card className="relative overflow-hidden">
      <div className={cx('mb-5 inline-flex rounded-2xl border p-3', tones[tone])}>{icon}</div>
      <div className="font-mono text-2xl font-semibold text-white">{value}</div>
      <div className="mt-2 text-sm font-medium text-slate-200">{title}</div>
      <p className="mt-2 text-xs leading-5 text-slate-500">{description}</p>
    </Card>
  );
}

function CapabilityCard({ title, enabled, enabledText, disabledText, description }: { title: string; enabled: boolean; enabledText: string; disabledText: string; description: string }) {
  return (
    <div className="rounded-2xl border border-slate-800 bg-slate-950/60 p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="font-medium text-white">{title}</div>
          <p className="mt-2 text-xs leading-5 text-slate-500">{description}</p>
        </div>
        <Badge value={enabled ? 'supported' : 'unsupported'}>{enabled ? enabledText : disabledText}</Badge>
      </div>
    </div>
  );
}

function CommandBlock({ command }: { command: string }) {
  return (
    <div className="rounded-2xl border border-slate-800 bg-slate-950 p-4">
      <div className="mb-3 flex items-center gap-2 text-sm text-slate-300">
        <Terminal className="h-4 w-4 text-blue-300" />
        推荐命令
      </div>
      <code className="block overflow-x-auto whitespace-nowrap rounded-xl bg-slate-900 px-3 py-3 font-mono text-xs text-blue-100">{command}</code>
    </div>
  );
}

function ChecklistItem({ ok, label }: { ok: boolean; label: string }) {
  return (
    <div className="flex items-start gap-3 rounded-2xl border border-slate-800 bg-slate-950/50 px-4 py-3 text-sm">
      {ok ? <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-300" /> : <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-300" />}
      <span className="text-slate-300">{label}</span>
    </div>
  );
}

function TroubleCard({ title, symptom, action }: { title: string; symptom: string; action: string }) {
  return (
    <div className="rounded-2xl border border-slate-800 bg-slate-950/60 p-4">
      <div className="flex items-center gap-2 text-sm font-medium text-white">
        <AlertTriangle className="h-4 w-4 text-amber-300" />
        {title}
      </div>
      <p className="mt-3 text-xs leading-5 text-slate-500">{symptom}</p>
      <p className="mt-3 rounded-xl border border-blue-400/20 bg-blue-500/10 px-3 py-2 text-xs leading-5 text-blue-100">{action}</p>
    </div>
  );
}

function InfoRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-slate-800/70 pb-3 last:border-0 last:pb-0">
      <span className="text-slate-500">{label}</span>
      {children}
    </div>
  );
}
