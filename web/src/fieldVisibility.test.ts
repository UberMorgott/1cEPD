import assert from 'node:assert/strict'
import { test } from 'node:test'
import { issueVisible, orphanVisible, requiredFields, type VisibilityState } from './fieldVisibility.ts'

function state(values: Record<string, unknown>, patch: Partial<VisibilityState> = {}): VisibilityState {
  return { revealed: false, touched: new Set(), focused: null, value: (key) => values[key], ...patch }
}

test('new request: empty untouched field stays quiet', () => {
  assert.equal(issueVisible('inn', state({ inn: '' })), false)
  assert.equal(issueVisible('companyName', state({})), false)
})

test('empty field shows its error after blur', () => {
  assert.equal(issueVisible('inn', state({ inn: '' }, { touched: new Set(['inn']) })), true)
})

test('submit attempt or saved draft reveals every field', () => {
  assert.equal(issueVisible('phone', state({ phone: '' }, { revealed: true })), true)
})

test('prefilled invalid value is flagged right away', () => {
  assert.equal(issueVisible('kpp', state({ kpp: '12' })), true)
  assert.equal(issueVisible('workplaces', state({ workplaces: 0 })), false)
})

test('value being typed is not flagged until the field is left', () => {
  assert.equal(issueVisible('inn', state({ inn: '77' }, { focused: 'inn' })), false)
  assert.equal(issueVisible('inn', state({ inn: '77' }, { focused: 'inn', touched: new Set(['inn']) })), true)
})

test('whole-request remarks wait for client inputs', () => {
  assert.equal(orphanVisible(false, ['', ' ', '']), false)
  assert.equal(orphanVisible(false, ['7700000000', '']), true)
  assert.equal(orphanVisible(true, ['']), true)
})

const blankRow = {
  companyName: '', inn: '', kpp: '', regNumber: '', responsible: '', phone: '',
  tariffCode: '', workplaces: 1, startDate: '01.10.26', deliveryType: '0', distributorCode: '',
}

test('required progress counts defaults as filled', () => {
  const fields = requiredFields({ partnerCode: '12345', responsible: '', email: '', row: blankRow })
  assert.equal(fields.length, 12)
  assert.deepEqual(fields.filter((f) => f.filled).map((f) => f.key), ['partnerCode', 'startDate', 'workplaces'])
})

test('sole proprietor needs no KPP; distributor delivery needs its code', () => {
  const ip = requiredFields({ partnerCode: '', responsible: '', email: '', row: { ...blankRow, inn: '770000000000' } })
  assert.ok(!ip.some((f) => f.key === 'kpp'))
  const viaDistributor = requiredFields({ partnerCode: '', responsible: '', email: '', row: { ...blankRow, deliveryType: '1' } })
  assert.ok(viaDistributor.some((f) => f.key === 'distributorCode'))
})
