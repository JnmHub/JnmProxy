export function cx(...values: Array<string | false | null | undefined>) {
  return values.filter(Boolean).join(' ');
}

export function statusLabel(value?: string) {
  const labels: Record<string, string> = {
    success: '成功',
    failed: '失败',
    never: '未刷新',
    alive: '可用',
    dead: '死亡',
    unknown: '未知',
    supported: '支持',
    unsupported: '不支持',
    error: '错误',
    all: '全部节点',
    group: '指定分组',
    node: '指定节点',
    random: '随机',
    fixed: '固定',
  };
  return value ? labels[value] ?? value : '—';
}
