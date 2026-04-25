import { useQuery } from '@tanstack/react-query';
import { getSingBoxStatus, getSystemHealth } from '../api/system';
import { Badge } from '../components/ui/Badge';
import { Card, CardHeader } from '../components/ui/Card';
import { LoadingState } from '../components/ui/LoadingState';
import { DataTable } from '../components/ui/Table';
import { formatTime } from '../utils/format';

export function SystemPage() {
  const healthQuery = useQuery({ queryKey: ['system', 'health'], queryFn: getSystemHealth });
  const singBoxQuery = useQuery({ queryKey: ['system', 'sing-box'], queryFn: getSingBoxStatus });
  const singBox = singBoxQuery.data;
  return (
    <div className="space-y-6">
      <div><p className="text-sm text-blue-300">System</p><h1 className="mt-2 text-3xl font-semibold text-white">系统状态</h1></div>
      <div className="grid gap-4 xl:grid-cols-2">
        <Card>
          <CardHeader title="API 健康" description="来自 /system/health" />
          {healthQuery.isLoading ? <LoadingState title="正在检查 API" /> : <div className="space-y-3 text-sm">
            <Row label="状态"><Badge value={healthQuery.data?.status === 'ok' ? 'alive' : 'dead'}>{healthQuery.data?.status ?? '加载中'}</Badge></Row>
            <Row label="时间"><span className="font-mono">{formatTime(healthQuery.data?.time)}</span></Row>
          </div>}
        </Card>
        <Card>
          <CardHeader title="sing-box" description="内嵌协议适配状态" />
          {singBoxQuery.isLoading ? <LoadingState title="正在读取 sing-box" /> : <div className="space-y-3 text-sm">
            <Row label="启用"><Badge value={singBox?.enabled ? 'supported' : 'unsupported'}>{singBox?.enabled ? '是' : '否'}</Badge></Row>
            <Row label="版本"><span className="font-mono">{singBox?.version ?? '—'}</span></Row>
            <Row label="配置版本"><span className="font-mono">{singBox?.config_version ?? '—'}</span></Row>
            <Row label="模式"><span className="font-mono">{singBox?.mode ?? '—'}</span></Row>
            <Row label="优先原生 HTTP/SOCKS"><Badge value={singBox?.prefer_native_http_socks ? 'supported' : 'unsupported'}>{singBox?.prefer_native_http_socks ? '是' : '否'}</Badge></Row>
            <Row label="Adapter 配置"><Badge value={singBox?.adapter_configured ? 'supported' : 'unsupported'}>{singBox?.adapter_configured ? '已配置' : '未配置'}</Badge></Row>
            <Row label="QUIC"><Badge value={singBox?.quic_enabled ? 'supported' : 'unsupported'}>{singBox?.quic_enabled ? '已启用' : '未启用'}</Badge></Row>
            <Row label="UDP"><Badge value={singBox?.enable_udp ? 'supported' : 'unsupported'}>{singBox?.enable_udp ? '已启用' : '未启用'}</Badge></Row>
            <Row label="最大引擎"><span className="font-mono">{singBox?.max_active_engines ?? '—'}</span></Row>
            <Row label="空闲超时"><span className="font-mono">{singBox?.engine_idle_timeout_seconds ? `${singBox.engine_idle_timeout_seconds}s` : '—'}</span></Row>
            <Row label="拨号超时"><span className="font-mono">{singBox?.engine_dial_timeout_seconds ? `${singBox.engine_dial_timeout_seconds}s` : '—'}</span></Row>
            <Row label="健康检查目标"><span className="font-mono text-xs">{singBox?.health_check_target ?? '—'}</span></Row>
            <Row label="许可证"><span className="text-slate-300">{singBox?.license ?? '—'}</span></Row>
          </div>}
        </Card>
      </div>
      <Card>
        <CardHeader title="支持协议" description="未启用 with_quic 时，Hysteria2/TUIC 不会进入支持状态。" />
        {singBoxQuery.isLoading ? <LoadingState title="正在加载协议列表" /> : (
          <DataTable columns={['协议']} empty={!singBox?.supported_protocols?.length}>
            {(singBox?.supported_protocols ?? []).map((protocol) => <tr key={protocol}><td className="px-4 py-3 font-mono text-blue-200">{protocol}</td></tr>)}
          </DataTable>
        )}
      </Card>
    </div>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return <div className="flex items-center justify-between gap-4 border-b border-slate-800/70 pb-3 last:border-0 last:pb-0"><span className="text-slate-500">{label}</span>{children}</div>;
}
