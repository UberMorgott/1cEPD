import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  autoMainReg,
  autoRegCandidates,
  extraRegOptions,
  regOptions,
  type RegProgram,
} from './regChoice.ts'

// Центр переработки (ИНН 4200000320): два регномера, условия выполнены у обоих.
const processing: RegProgram[] = [
  { regNumber: '200001900001', program: 'Технологическая платформа', hasAccess: true },
  { regNumber: '200001900001', program: '1С:Бухгалтерия предприятия', hasAccess: true },
  { regNumber: '200002100002', program: '1С:Зарплата и управление персоналом', hasAccess: true },
]

test('two reg numbers: dropdown with both, first preselected when field is empty', () => {
  const candidates = autoRegCandidates(processing)
  assert.deepEqual(candidates, ['200001900001', '200002100002'])
  assert.equal(autoMainReg('', candidates), '200001900001')
  assert.deepEqual(regOptions(processing), [
    { value: '200001900001', label: '200001900001 — 1С:Бухгалтерия предприятия' },
    { value: '200002100002', label: '200002100002 — 1С:Зарплата и управление персоналом' },
  ])
})

test('one reg number fills the field', () => {
  const one = processing.slice(0, 2)
  assert.equal(autoMainReg('', autoRegCandidates(one)), '200001900001')
  assert.equal(regOptions(one).length, 1)
})

test('no programs leaves the field empty', () => {
  assert.deepEqual(autoRegCandidates([]), [])
  assert.equal(autoMainReg('', []), '')
})

test('value the user typed is kept', () => {
  assert.equal(autoMainReg('123', autoRegCandidates(processing)), '123')
})

test('programs with support conditions met come first', () => {
  const mixed: RegProgram[] = [
    { regNumber: '111', program: 'Старая', hasAccess: false },
    { regNumber: '222', program: 'Действующая', hasAccess: true },
  ]
  assert.deepEqual(autoRegCandidates(mixed), ['222'])
  assert.deepEqual(regOptions(mixed).map((o) => o.value), ['222', '111'])
  // Ни у одной условия не выполнены — выбираем из всех.
  const none = mixed.map((p) => ({ ...p, hasAccess: false }))
  assert.deepEqual(autoRegCandidates(none), ['111', '222'])
})

test('main reg number from the previous request leads, extras follow programs', () => {
  const values = regOptions(processing, '200002100002', ['999', '']).map((o) => o.value)
  assert.deepEqual(values, ['200002100002', '200001900001', '999'])
})

test('null fields from JSON cards are treated as empty', () => {
  assert.deepEqual(regOptions(null, null, null), [])
  assert.deepEqual(autoRegCandidates(null), [])
})

test('extra reg numbers offer every number except the main one', () => {
  const options = regOptions(processing)
  assert.deepEqual(
    extraRegOptions(options, '200001900001').map((o) => o.value),
    ['200002100002'],
  )
})
