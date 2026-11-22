// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('../src/api', () => ({ listUserUsage: vi.fn(), createGrant: vi.fn() }))

import CreditUsage from '../src/components/CreditUsage.vue'
import { createGrant, listUserUsage } from '../src/api'
import { useToasts } from '../src/composables/useToasts'

const USERS = [
  { org: 'acme', user: 'alice', monthlyAllowance: 300, monthlyUsed: 80, grants: 0, balance: 220 },
  { org: 'acme', user: 'bob', monthlyAllowance: 300, monthlyUsed: 300, grants: 70, balance: 70 },
]

async function mountUsage(page = { users: USERS, truncated: false }) {
  listUserUsage.mockResolvedValue(page)
  const wrapper = mount(CreditUsage, { props: { service: 'mesher' } })
  await flushPromises()
  return wrapper
}

describe('CreditUsage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useToasts().toasts.value = []
  })

  it('lists users with their usage', async () => {
    listUserUsage.mockReturnValueOnce(new Promise(() => {}))
    expect(mount(CreditUsage, { props: { service: 'mesher' } }).find('[data-test="usage-loading"]').exists()).toBe(true)

    const wrapper = await mountUsage()
    expect(listUserUsage).toHaveBeenCalledWith('mesher', '')
    const alice = wrapper.find('[data-test="usage-acme-alice"]')
    expect(alice.text()).toContain('80')
    expect(alice.text()).toContain('/ 300')
    expect(alice.text()).toContain('220')
    expect(wrapper.find('[data-test="usage-acme-bob"] .text-rose-600').text()).toBe('300')
    expect(wrapper.find('[data-test="usage-truncated"]').exists()).toBe(false)
  })

  it('filters by org, says when nothing matches, and warns when cut short', async () => {
    const wrapper = await mountUsage({ users: [], truncated: false })
    expect(wrapper.find('[data-test="usage-empty"]').text()).toContain('No user has credits yet')

    listUserUsage.mockResolvedValueOnce({ users: [], truncated: false })
    await wrapper.find('[data-test="usage-org"]').setValue(' zed ')
    await wrapper.find('[data-test="usage-filter"]').trigger('submit')
    await flushPromises()
    expect(listUserUsage).toHaveBeenLastCalledWith('mesher', 'zed')
    expect(wrapper.find('[data-test="usage-empty"]').text()).toBe('No user of “zed” has credits.')

    listUserUsage.mockResolvedValueOnce({ users: USERS, truncated: true })
    await wrapper.find('[data-test="usage-filter"]').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[data-test="usage-truncated"]').text()).toContain('first 2 users')
  })

  it('reports a failed load', async () => {
    const err = new Error('boom')
    listUserUsage.mockRejectedValueOnce(err)
    const wrapper = mount(CreditUsage, { props: { service: 'mesher' } })
    await flushPromises()
    expect(wrapper.emitted('error')).toEqual([[err]])
  })

  it('grants credits to a picked user', async () => {
    const wrapper = await mountUsage()
    expect(wrapper.find('[data-test="grant-submit"]').attributes('disabled')).toBeDefined()
    await wrapper.find('[data-test="usage-acme-bob"]').trigger('click')
    expect(wrapper.find('[data-test="grant-org"]').element.value).toBe('acme')
    expect(wrapper.find('[data-test="grant-user"]').element.value).toBe('bob')

    await wrapper.find('[data-test="grant-credits"]').setValue('50')
    createGrant.mockResolvedValueOnce({ user: 'bob', credits: 50, expiresAt: '2026-12-23T00:00:00Z' })
    await wrapper.find('[data-test="grant-form"]').trigger('submit')
    await flushPromises()
    expect(createGrant).toHaveBeenCalledWith({ service: 'mesher', org: 'acme', user: 'bob', credits: 50 })
    expect(useToasts().toasts.value.at(-1).message).toContain('Granted 50 credits to bob')
    expect(wrapper.find('[data-test="grant-credits"]').element.value).toBe('')
    expect(listUserUsage).toHaveBeenCalledTimes(2)
  })

  it('checks the credits and reports a failed grant', async () => {
    const wrapper = await mountUsage()
    await wrapper.find('[data-test="grant-org"]').setValue('acme')
    await wrapper.find('[data-test="grant-user"]').setValue('new')
    for (const credits of ['0', '1.5', 'x']) {
      await wrapper.find('[data-test="grant-credits"]').setValue(credits)
      await wrapper.find('[data-test="grant-form"]').trigger('submit')
    }
    expect(createGrant).not.toHaveBeenCalled()
    expect(useToasts().toasts.value.filter((t) => t.type === 'error')).toHaveLength(3)

    await wrapper.find('[data-test="grant-credits"]').setValue('5')
    const err = new Error('nope')
    createGrant.mockRejectedValueOnce(err)
    await wrapper.find('[data-test="grant-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.emitted('error')).toEqual([[err]])
    expect(wrapper.find('[data-test="grant-credits"]').element.value).toBe('5')
  })
})
