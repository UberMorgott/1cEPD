/** Версия для людей: «v0.1.0», у сборки без версии — как есть («dev»). */
export function versionLabel(version: string | undefined): string {
  const v = version || 'dev'
  return /^\d+\.\d+\.\d+/.test(v) ? `v${v}` : v
}

/** Процент загрузки или null, если размер неизвестен. */
export function downloadPercent(downloaded: number | undefined, size: number | undefined): number | null {
  if (!size || size <= 0) return null
  return Math.min(100, Math.floor(((downloaded ?? 0) * 100) / size))
}

/** Мегабайты с одним знаком: «3.4». */
export function megabytes(bytes: number | undefined): string {
  return ((bytes ?? 0) / (1 << 20)).toFixed(1)
}

/**
 * Поднялась ли новая сборка после перезапуска: сервер ответил и версия
 * уже не та, что была до обновления.
 */
export function restartedWithNewVersion(before: string, health: { version?: string } | null): boolean {
  return !!health?.version && health.version !== before
}
