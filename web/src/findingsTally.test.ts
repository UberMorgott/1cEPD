import assert from 'node:assert/strict'
import { test } from 'node:test'
import { findingsTally } from './findingsTally.ts'

test('findings counter counts clients, not findings', () => {
  const tally = findingsTally([
    { clients: [{ key: 'a' }], item: { confidence: 'high' } },
    { clients: [{ key: 'a' }], item: { confidence: 'low' } },
    { clients: [{ key: 'b' }, { key: 'a' }], item: { confidence: 'medium' } },
    { clients: [{ key: 'c' }], item: { confidence: 'high' } },
  ])
  assert.deepEqual(tally, { clients: 3, findings: 4, urgent: 2 })
})

test('a finding without a client card counts as its own client', () => {
  const tally = findingsTally([
    { clients: [], item: { confidence: 'low' } },
    { clients: [], item: { confidence: 'low' } },
    { clients: [{ key: 'a' }], item: { confidence: 'low' } },
  ])
  assert.deepEqual(tally, { clients: 3, findings: 3, urgent: 0 })
})

test('no findings', () => {
  assert.deepEqual(findingsTally([]), { clients: 0, findings: 0, urgent: 0 })
})
