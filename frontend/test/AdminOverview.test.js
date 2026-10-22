// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('../src/api', () => ({ fetchAdminStats: vi.fn() }))

import AdminOverview from '../src/components/AdminOverview.vue'
import { fetchAdminStats } from '../src/api'

describe('AdminOverview', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows the totals and the daily charts', async () => {
    fetchAdminStats.mockResolvedValue({
      users: 1234,
      signups: 2,
      days: [
        { date: '2026-10-20', signups: 2 },
        { date: '2026-10-21', signups: 0 },
      ],
    })
    const wrapper = mount(AdminOverview)
    expect(wrapper.text()).toContain('Loading...')
    await flushPromises()

    const stat = (name) => wrapper.find(`[data-test="admin-stat-${name}"]`).text()
    expect(stat('users')).toBe((1234).toLocaleString())
    expect(stat('signups')).toBe('2')
    expect(wrapper.find('[data-test="admin-chart-signups"]').text()).toContain('Sign-ups')
    expect(wrapper.text()).not.toContain('Loading...')
  })

  it('reports a failed load and shows zeros', async () => {
    const err = new Error('Failed to load stats (500)')
    fetchAdminStats.mockRejectedValue(err)
    const wrapper = mount(AdminOverview)
    await flushPromises()
    expect(wrapper.emitted('error')).toEqual([[err]])
    expect(wrapper.find('[data-test="admin-stat-users"]').text()).toBe('0')
  })
})
