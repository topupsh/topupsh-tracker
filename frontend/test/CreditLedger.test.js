// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('../src/api', () => ({ listLedger: vi.fn(), listServices: vi.fn() }))

import CreditLedger from '../src/components/CreditLedger.vue'
import { listLedger, listServices } from '../src/api'

const entry = (id, extra = {}) => ({
  id,
  service: 'mesher',
  org: 'acme',
  user: 'bob',
  kind: 'spend',
  amount: -30,
  balanceAfter: 70,
  actor: 'mesher',
  operation: 'tool:fetch',
  createdAt: '2026-11-22T01:00:00Z',
  ...extra,
})

function deferred() {
  let resolve
  const promise = new Promise((r) => {
    resolve = r
  })
  return { promise, resolve }
}

describe('CreditLedger', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listServices.mockResolvedValue([{ name: 'mesher' }, { name: 'tmphost' }])
  })

  it('lists entries, newest first, and loads more', async () => {
    listLedger.mockResolvedValueOnce({ entries: [entry('e3'), entry('e2', { kind: 'grant', amount: 100, actor: 'root', operation: '' })], nextCursor: 'e2' })
    const wrapper = mount(CreditLedger)
    expect(wrapper.find('[data-test="ledger-loading"]').exists()).toBe(true)
    await flushPromises()
    expect(listLedger).toHaveBeenCalledWith({ service: '', org: '', user: '' })
    expect(wrapper.findAll('[data-test="ledger-service"] option').map((o) => o.text())).toEqual(['All services', 'mesher', 'tmphost'])

    const spend = wrapper.find('[data-test="ledger-e3"]')
    expect(spend.text()).toContain('tool:fetch')
    expect(spend.find('.text-rose-600').text()).toBe('-30')
    const grant = wrapper.find('[data-test="ledger-e2"]')
    expect(grant.text()).not.toContain('tool:')
    expect(grant.find('.text-emerald-600').text()).toBe('+100')
    expect(grant.text()).toContain('root')

    const more = deferred()
    listLedger.mockReturnValueOnce(more.promise)
    await wrapper.find('[data-test="ledger-more"]').trigger('click')
    expect(listLedger).toHaveBeenLastCalledWith({ service: '', org: '', user: '', cursor: 'e2' })
    expect(wrapper.find('[data-test="ledger-more"]').attributes('disabled')).toBeDefined()
    more.resolve({ entries: [entry('e1', { kind: 'mystery' })] })
    await flushPromises()
    expect(wrapper.find('[data-test="ledger-e1"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="ledger-more"]').exists()).toBe(false)
  })

  it('filters by service, org and user', async () => {
    listLedger.mockResolvedValue({ entries: [] })
    const wrapper = mount(CreditLedger)
    await flushPromises()
    expect(wrapper.find('[data-test="ledger-empty"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="ledger-user"]').attributes('disabled')).toBeDefined()

    await wrapper.find('[data-test="ledger-service"]').setValue('mesher')
    await wrapper.find('[data-test="ledger-org"]').setValue(' acme ')
    expect(wrapper.find('[data-test="ledger-user"]').attributes('disabled')).toBeUndefined()
    await wrapper.find('[data-test="ledger-user"]').setValue('bob ')
    await wrapper.find('[data-test="ledger-filters"]').trigger('submit')
    await flushPromises()
    expect(listLedger).toHaveBeenLastCalledWith({ service: 'mesher', org: 'acme', user: 'bob' })
  })

  it('reports failed loads', async () => {
    const err = new Error('boom')
    listServices.mockRejectedValueOnce(err)
    listLedger.mockRejectedValueOnce(err)
    const wrapper = mount(CreditLedger)
    await flushPromises()
    expect(wrapper.emitted('error')).toEqual([[err], [err]])

    listLedger.mockResolvedValueOnce({ entries: [entry('e1')], nextCursor: 'e1' })
    await wrapper.find('[data-test="ledger-filters"]').trigger('submit')
    await flushPromises()
    listLedger.mockRejectedValueOnce(err)
    await wrapper.find('[data-test="ledger-more"]').trigger('click')
    await flushPromises()
    expect(wrapper.emitted('error')).toHaveLength(3)
  })
})
