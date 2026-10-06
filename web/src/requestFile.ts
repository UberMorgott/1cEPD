import type { ItsRequest } from './api/client'

/** Имя файла заявки, как его собирает сервер: ip<код партнёра>.xls. */
export function requestFileName(request: Pick<ItsRequest, 'partnerCode'>): string {
  return `ip${request.partnerCode}.xls`
}

/**
 * Собирает файл заявки на сервере и отдаёт браузеру на сохранение. Возвращает
 * пустую строку при успехе, иначе — объяснение отказа от сервера.
 *
 * Скачивание идёт через blob: fetch несёт cookie сессии, а браузер всё равно
 * показывает диалог сохранения по ссылке с download.
 */
export async function downloadRequestFile(request: ItsRequest): Promise<string> {
  const response = await fetch('/api/its/download', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  })
  if (!response.ok) {
    // Сервер объясняет отказ в теле: {error: {message}}. Его и показываем —
    // общая фраза не говорит, что исправить.
    const body = (await response.json().catch(() => null)) as {
      error?: { message?: string }
    } | null
    return body?.error?.message ?? 'Не удалось собрать файл.'
  }

  const blob = await response.blob()
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = requestFileName(request)
  link.click()
  // Сразу отзывать нельзя: браузер начинает загрузку после обработчика клика.
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  return ''
}

/** Состояние заявки по отметкам: отправлена важнее выгруженной. */
export type RequestState = 'draft' | 'exported' | 'sent'

export function requestState(item: { exportedAt?: string; sentAt?: string }): RequestState {
  if (item.sentAt) return 'sent'
  if (item.exportedAt) return 'exported'
  return 'draft'
}

export const requestStateLabels: Record<RequestState, string> = {
  draft: 'Черновик',
  exported: 'Выгружена',
  sent: 'Отправлена',
}
