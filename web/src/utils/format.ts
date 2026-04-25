import dayjs from 'dayjs';

export function formatTime(value?: string) {
  if (!value) return '—';
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss') : value;
}

export function maskURL(value?: string) {
  if (!value) return '—';
  if (value.length <= 24) return value;
  return `${value.slice(0, 16)}…${value.slice(-8)}`;
}

export function compactNumber(value?: number) {
  if (value === undefined || Number.isNaN(value)) return '0';
  return new Intl.NumberFormat('zh-CN').format(value);
}
