import assert from 'node:assert/strict'
import { test } from 'node:test'
import { downloadPercent, megabytes, restartedWithNewVersion, versionLabel } from './update.ts'

test('versionLabel: релиз с «v», dev как есть', () => {
  assert.equal(versionLabel('0.1.0'), 'v0.1.0')
  assert.equal(versionLabel('dev'), 'dev')
  assert.equal(versionLabel(undefined), 'dev')
})

test('downloadPercent: неизвестный размер — null, не больше 100', () => {
  assert.equal(downloadPercent(5, 0), null)
  assert.equal(downloadPercent(5, 10), 50)
  assert.equal(downloadPercent(20, 10), 100)
})

test('megabytes: один знак', () => {
  assert.equal(megabytes(3 * 1024 * 1024 + 400 * 1024), '3.4')
})

test('restartedWithNewVersion: ждём другую версию', () => {
  assert.equal(restartedWithNewVersion('0.0.1', null), false)
  assert.equal(restartedWithNewVersion('0.0.1', { version: '0.0.1' }), false)
  assert.equal(restartedWithNewVersion('0.0.1', { version: '0.1.0' }), true)
})
