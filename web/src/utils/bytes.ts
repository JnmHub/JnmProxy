export function formatBytes(value?: number) {
  if (!value || value <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  let size = value;
  let index = 0;
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024;
    index++;
  }
  return `${size.toFixed(size >= 10 || index === 0 ? 0 : 1)} ${units[index]}`;
}

export function usagePercent(used?: number, total?: number) {
  if (!used || !total || total <= 0) return 0;
  return Math.min(100, Math.round((used / total) * 100));
}
