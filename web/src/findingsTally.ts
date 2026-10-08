/**
 * Счёт находок ЭДО для счётчика Клиентов: счётчики экрана считают клиентов,
 * поэтому и «Находки ЭДО» — клиенты с активными находками, а число самих
 * находок идёт вторым планом. Находка без карточки клиента считается отдельным
 * клиентом: список находок её покажет.
 */
export interface FindingEntry {
  clients: { key: string }[]
  item: { confidence: string }
}

export interface FindingsTally {
  /** Клиентов с активными находками (плюс находки без карточки). */
  clients: number
  /** Активных находок. */
  findings: number
  /** Срочных находок. */
  urgent: number
}

export function findingsTally(entries: readonly FindingEntry[]): FindingsTally {
  const keys = new Set<string>()
  let unmatched = 0
  let urgent = 0
  for (const entry of entries) {
    if (entry.clients.length) for (const client of entry.clients) keys.add(client.key)
    else unmatched++
    if (entry.item.confidence === 'high') urgent++
  }
  return { clients: keys.size + unmatched, findings: entries.length, urgent }
}
