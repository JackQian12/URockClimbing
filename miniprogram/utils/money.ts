export function formatCent(value: number): string {
  const cent = Math.max(0, Math.trunc(value))
  return `¥${Math.floor(cent / 100)}.${String(cent % 100).padStart(2, '0')}`
}

