// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

vi.mock('../src/api', () => ({ fetchViewerBalance: vi.fn(), listViewerLedger: vi.fn() }))

import BalanceView from '../src/views/BalanceView.vue'
import { fetchViewerBalance, listViewerLedger } from '../src/api'

const BALANCE = {
  service: 'mesher',
  org: 'acme',
  user: 'bob',
  limited: true,
  balance: 70,
  available: 70,
  monthly: { limit: 300, used: 300, remaining: 0 },
  orgUsage: { limit: 1000, userLimit: 300, used: 425, remaining: 575 },
  grants: [{ id: 'g1', credits: 100, remaining: 70, createdAt: '2026-11-22T01:00:00Z', expiresAt: '2026-12-23T00:00:00Z' }],
  renewsAt: '2026-12-01T00:00:00Z',
}
const entry = (id, extra = {}) => ({ id, kind: 'spend', amount: -30, balanceAfter: 70, operation: 'tool:fetch', createdAt: '2026-11-22T01:00:00Z', ...extra })

async function mountAt(path) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/balance', component: BalanceView }] })
  await router.push(path)
  await router.isReady()
  const wrapper = mount(BalanceView, { global: { plugins: [router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('BalanceView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('takes the token from the link, hides it, and shows the balance and history', async () => {
    fetchViewerBalance.mockResolvedValueOnce(BALANCE)
    listViewerLedger.mockResolvedValueOnce({
      entries: [entry('e2'), entry('e1', { kind: 'renewal', amount: 300, balanceAfter: 300, operation: 'tool:search' }), entry('e0', { kind: 'other', operation: '' })],
      nextCursor: '',
    })
    const { wrapper, router } = await mountAt('/balance?x=1#token=tok')
    expect(fetchViewerBalance).toHaveBeenCalledWith('tok')
    expect(listViewerLedger).toHaveBeenCalledWith('tok')
    expect(sessionStorage.getItem('topupsh.viewToken')).toBe('tok')
    expect(router.currentRoute.value.hash).toBe('')
    expect(router.currentRoute.value.query).toEqual({ x: '1' })

    expect(wrapper.find('[data-test="balance-subject"]').text()).toContain('mesher · bob in acme')
    expect(wrapper.find('[data-test="balance-unlimited"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="balance-total"]').text()).toBe('70')
    expect(wrapper.find('[data-test="balance-available"]').text()).toBe('70')
    expect(wrapper.find('[data-test="balance-monthly"]').text()).toContain('left of 300 · 300 used')
    expect(wrapper.find('[data-test="balance-monthly-bar"]').attributes('style')).toContain('width: 100%')
    expect(wrapper.find('[data-test="balance-monthly-bar"]').classes()).toContain('bg-rose-500')
    expect(wrapper.find('[data-test="balance-org"]').text()).toContain('575 of its 1,000')
    expect(wrapper.find('[data-test="grant-g1"]').text()).toContain('70 of 100 left')
    expect(wrapper.find('[data-test="grant-g1"]').text()).toContain('Expires')

    const spend = wrapper.find('[data-test="entry-e2"]')
    expect(spend.text()).toContain('Spent · tool:fetch')
    expect(spend.find('.text-rose-600').text()).toBe('-30')
    const renewal = wrapper.find('[data-test="entry-e1"]')
    expect(renewal.text()).toContain('Monthly credits')
    expect(renewal.text()).not.toContain('tool:search')
    expect(renewal.find('.text-emerald-600').text()).toBe('+300')
    expect(wrapper.find('[data-test="entry-e0"]').text()).toContain('other')
    expect(wrapper.find('[data-test="balance-more"]').exists()).toBe(false)
  })

  it('reuses a stored token, shows empty states, a part-used month and the switch being off', async () => {
    sessionStorage.setItem('topupsh.viewToken', 'stored')
    fetchViewerBalance.mockResolvedValueOnce({ ...BALANCE, limited: false, monthly: { limit: 300, used: 45, remaining: 255 }, grants: [] })
    listViewerLedger.mockResolvedValueOnce({ entries: [] })
    const { wrapper } = await mountAt('/balance')
    expect(fetchViewerBalance).toHaveBeenCalledWith('stored')
    expect(wrapper.find('[data-test="balance-unlimited"]').text()).toContain("mesher doesn't limit credits")
    expect(wrapper.find('[data-test="balance-monthly-bar"]').attributes('style')).toContain('width: 15%')
    expect(wrapper.find('[data-test="balance-grants-empty"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="balance-history-empty"]').exists()).toBe(true)
  })

  it('shows a full bar for a zero allowance', async () => {
    sessionStorage.setItem('topupsh.viewToken', 'stored')
    fetchViewerBalance.mockResolvedValueOnce({ ...BALANCE, monthly: { limit: 0, used: 0, remaining: 0 } })
    listViewerLedger.mockResolvedValueOnce({ entries: [] })
    const { wrapper } = await mountAt('/balance')
    expect(wrapper.find('[data-test="balance-monthly-bar"]').attributes('style')).toContain('width: 100%')
  })

  it('loads more history', async () => {
    fetchViewerBalance.mockResolvedValueOnce(BALANCE)
    listViewerLedger.mockResolvedValueOnce({ entries: [entry('e2')], nextCursor: 'e2' })
    const { wrapper } = await mountAt('/balance#token=tok')
    listViewerLedger.mockResolvedValueOnce({ entries: [entry('e1')] })
    await wrapper.find('[data-test="balance-more"]').trigger('click')
    await flushPromises()
    expect(listViewerLedger).toHaveBeenLastCalledWith('tok', 'e2')
    expect(wrapper.find('[data-test="entry-e1"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="balance-more"]').exists()).toBe(false)
  })

  it('asks to open the page from the app without a token', async () => {
    const { wrapper } = await mountAt('/balance#other=1')
    expect(wrapper.find('[data-test="balance-missing"]').exists()).toBe(true)
    expect(fetchViewerBalance).not.toHaveBeenCalled()
  })

  it('says the link expired on 401 and forgets the token', async () => {
    fetchViewerBalance.mockRejectedValueOnce(Object.assign(new Error('invalid token'), { status: 401 }))
    listViewerLedger.mockResolvedValueOnce({ entries: [] })
    const { wrapper } = await mountAt('/balance#token=old')
    expect(wrapper.find('[data-test="balance-expired"]').exists()).toBe(true)
    expect(sessionStorage.getItem('topupsh.viewToken')).toBeNull()
  })

  it('shows other errors, also when loading more', async () => {
    fetchViewerBalance.mockRejectedValueOnce(new Error('Failed to load your balance (500)'))
    listViewerLedger.mockResolvedValueOnce({ entries: [] })
    const { wrapper } = await mountAt('/balance#token=tok')
    expect(wrapper.find('[data-test="balance-error-message"]').text()).toBe('Failed to load your balance (500)')

    fetchViewerBalance.mockResolvedValueOnce(BALANCE)
    listViewerLedger.mockResolvedValueOnce({ entries: [entry('e2')], nextCursor: 'e2' })
    const again = (await mountAt('/balance#token=tok')).wrapper
    listViewerLedger.mockRejectedValueOnce(Object.assign(new Error('expired'), { status: 401 }))
    await again.find('[data-test="balance-more"]').trigger('click')
    await flushPromises()
    expect(again.find('[data-test="balance-expired"]').exists()).toBe(true)
  })

  it('works without session storage', async () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
      throw new Error('blocked')
    })
    const { wrapper } = await mountAt('/balance')
    expect(wrapper.find('[data-test="balance-missing"]').exists()).toBe(true)

    fetchViewerBalance.mockRejectedValueOnce(Object.assign(new Error('expired'), { status: 401 }))
    listViewerLedger.mockResolvedValueOnce({ entries: [] })
    const linked = (await mountAt('/balance#token=tok')).wrapper
    expect(fetchViewerBalance).toHaveBeenCalledWith('tok')
    expect(linked.find('[data-test="balance-expired"]').exists()).toBe(true)
  })
})
