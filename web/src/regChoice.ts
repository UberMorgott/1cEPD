/**
 * Выбор регистрационного номера в заявке из программ Личного кабинета клиента.
 *
 * Только данные самого клиента: его прошлая заявка и проверка программ по его
 * логину. База абонентов партнёра сюда не попадает — у абонентов агрегаторов
 * там регномера других организаций.
 */

/** Программа из проверки в 1С: только то, что нужно для выбора регномера. */
export interface RegProgram {
  regNumber: string
  program: string
  hasAccess: boolean
}

export interface RegOption {
  value: string
  label: string
}

/** Технологическая платформа идёт строкой при каждой программе — в подписи регномера она лишняя. */
const platform = 'Технологическая платформа'

/**
 * Регномера клиента для списка выбора, без повторов: основной из прошлой
 * заявки, затем программы с выполненными условиями, прочие программы и
 * остальные регномера прошлой заявки. В подписи — названия программ.
 */
export function regOptions(
  programs?: RegProgram[] | null,
  main?: string | null,
  extras?: string[] | null,
): RegOption[] {
  // Карточки из JSON: поле без значения приходит как null.
  const list = programs ?? []
  const names = new Map<string, string[]>()
  const add = (number: string | null | undefined, program?: string) => {
    const value = (number ?? '').trim()
    if (!value) return
    const known = names.get(value) ?? []
    if (program && !known.includes(program)) known.push(program)
    names.set(value, known)
  }
  add(main)
  const ordered = [...list.filter((p) => p.hasAccess), ...list.filter((p) => !p.hasAccess)]
  ordered.forEach((program) => add(program.regNumber, program.program))
  extras?.forEach((number) => add(number))
  return [...names].map(([value, found]) => {
    const products = found.filter((name) => name !== platform)
    const shown = products.length ? products : found
    return { value, label: shown.length ? `${value} — ${shown.join(', ')}` : value }
  })
}

/**
 * Регномера, из которых форма сама выбирает основной: программы с
 * выполненными условиями сопровождения, а если таких нет — все программы.
 */
export function autoRegCandidates(programs?: RegProgram[] | null): string[] {
  const list = programs ?? []
  const withAccess = list.filter((program) => program.hasAccess)
  const source = withAccess.length ? withAccess : list
  return [...new Set(source.map((program) => program.regNumber.trim()).filter(Boolean))]
}

/**
 * Значение поля «Регистрационный номер» после проверки программ: введённое
 * не трогаем, пустое получает первый из кандидатов (единственный — тем более).
 */
export function autoMainReg(current: string, candidates: string[]): string {
  if (current.trim()) return current
  return candidates[0] ?? ''
}

/** Варианты для «Других программ»: регномера клиента, кроме основного. */
export function extraRegOptions(options: RegOption[], main: string): RegOption[] {
  const chosen = main.trim()
  return options.filter((option) => option.value !== chosen)
}
