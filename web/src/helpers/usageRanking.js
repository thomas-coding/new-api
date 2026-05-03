export function formatTokenMillions(value) {
  const tokens = Number(value || 0);
  if (!Number.isFinite(tokens) || tokens <= 0) {
    return '0M';
  }

  const millions = tokens / 1000000;
  if (millions < 0.01) {
    return '<0.01M';
  }
  if (millions >= 100) {
    return `${Math.round(millions).toLocaleString()}M`;
  }
  if (millions >= 10) {
    return `${millions.toLocaleString(undefined, {
      maximumFractionDigits: 1,
    })}M`;
  }
  return `${millions.toLocaleString(undefined, {
    maximumFractionDigits: 2,
  })}M`;
}
