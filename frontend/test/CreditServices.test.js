// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('../src/api', () => ({
  listServices: vi.fn(),
  createServiceKey: vi.fn(),
  revokeServiceKey: vi.fn(),
  setServiceEnabled: vi.fn(),
  setServiceDefaults: vi.fn(),
}))

import CreditServices from '../src/components/CreditServices.vue'
import { createServiceKey, listServices, revokeServiceKey, setServiceDefaults, setServiceEnabled } from '../src/api'
import { useToasts } from '../src/composables/useToasts'

const AT = '2026-11-21T12:00:00Z'
const mesher = (extra = {}) => ({
  name: 'mesher',
  enabled: false,
  defaultOrgLimit: 200,
  defaultUserLimit: 50,
  createdAt: AT,
  apiKeys: [
    { id: 'k2', service: 'mesher', prefix: 'tpk_new', createdAt: AT },
    { id: 'k1', service: 'mesher', prefix: 'tpk_old', createdAt: AT, revokedAt: AT },
  ],
  ...extra,
})
const tmphost = () => ({ name: 'tmphost', enabled: true, defaultOrgLimit: 0, defaultUserLimit: null, createdAt: AT, apiKeys: [] })

async function mountServices(services = [mesher(), tmphost()]) {
  listServices.mockResolvedValue(services)
  const wrapper = mount(CreditServices, { attachTo: document.body })
  await flushPromises()
  return wrapper
}

const toasts = () => useToasts().toasts.value.map((t) => [t.type, t.message])

describe('CreditServices', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useToasts().toasts.value = []
  })
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('lists services with their switch, defaults and keys', async () => {
    listServices.mockReturnValueOnce(new Promise(() => {}))
    const loading = mount(CreditServices)
    expect(loading.find('[data-test="services-loading"]').exists()).toBe(true)

    const wrapper = await mountServices()
    const m = wrapper.find('[data-test="service-mesher"]')
    expect(m.text()).toContain('Limiting off')
    expect(wrapper.find('[data-test="service-switch-mesher"]').attributes('aria-checked')).toBe('false')
    expect(wrapper.find('[data-test="service-switch-tmphost"]').attributes('aria-checked')).toBe('true')
    expect(wrapper.find('[data-test="defaults-org-mesher"]').element.value).toBe('200')
    expect(wrapper.find('[data-test="defaults-user-mesher"]').element.value).toBe('50')
    expect(wrapper.find('[data-test="defaults-user-tmphost"]').element.value).toBe('')
    expect(wrapper.find('[data-test="key-k2"]').text()).toContain('tpk_new…')
    expect(wrapper.find('[data-test="key-revoke-k2"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="key-k1"]').text()).toContain('revoked')
    expect(wrapper.find('[data-test="key-revoke-k1"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="service-tmphost"]').text()).toContain('No keys.')
  })

  it('says when there are no services', async () => {
    const wrapper = await mountServices([])
    expect(wrapper.find('[data-test="services-empty"]').exists()).toBe(true)
  })

  it('reports a failed load', async () => {
    const err = new Error('boom')
    listServices.mockRejectedValueOnce(err)
    const wrapper = mount(CreditServices)
    await flushPromises()
    expect(wrapper.emitted('error')).toEqual([[err]])
  })

  it('switches limiting on and off', async () => {
    const wrapper = await mountServices()
    setServiceEnabled.mockResolvedValueOnce({ name: 'mesher', enabled: true })
    await wrapper.find('[data-test="service-switch-mesher"]').trigger('click')
    await flushPromises()
    expect(setServiceEnabled).toHaveBeenCalledWith('mesher', true)
    expect(wrapper.find('[data-test="service-switch-mesher"]').attributes('aria-checked')).toBe('true')
    expect(toasts()).toContainEqual(['success', 'Credit limiting is on for mesher'])

    setServiceEnabled.mockResolvedValueOnce({ name: 'tmphost', enabled: false })
    await wrapper.find('[data-test="service-switch-tmphost"]').trigger('click')
    await flushPromises()
    expect(toasts()).toContainEqual(['success', 'Credit limiting is off for tmphost'])

    const err = new Error('nope')
    setServiceEnabled.mockRejectedValueOnce(err)
    await wrapper.find('[data-test="service-switch-mesher"]').trigger('click')
    await flushPromises()
    expect(wrapper.emitted('error')).toEqual([[err]])
    expect(wrapper.find('[data-test="service-switch-mesher"]').attributes('aria-checked')).toBe('true')
  })

  it('saves default limits and checks them first', async () => {
    const wrapper = await mountServices()
    await wrapper.find('[data-test="defaults-org-mesher"]').setValue('300')
    await wrapper.find('[data-test="defaults-user-mesher"]').setValue('')
    setServiceDefaults.mockResolvedValueOnce({ name: 'mesher', defaultOrgLimit: 300, defaultUserLimit: null })
    await wrapper.find('[data-test="service-defaults-mesher"]').trigger('submit')
    await flushPromises()
    expect(setServiceDefaults).toHaveBeenCalledWith('mesher', { orgLimit: 300, userLimit: null })
    expect(toasts()).toContainEqual(['success', 'Saved the default limits of mesher'])

    for (const [org, user] of [['', '1'], ['x', ''], ['1', '-2']]) {
      await wrapper.find('[data-test="defaults-org-mesher"]').setValue(org)
      await wrapper.find('[data-test="defaults-user-mesher"]').setValue(user)
      await wrapper.find('[data-test="service-defaults-mesher"]').trigger('submit')
    }
    expect(setServiceDefaults).toHaveBeenCalledTimes(1)
    expect(toasts().filter(([type]) => type === 'error')).toHaveLength(3)
  })

  it('creates a service with its first key and shows the key once', async () => {
    const wrapper = await mountServices([])
    expect(wrapper.find('[data-test="service-new-submit"]').attributes('disabled')).toBeDefined()
    await wrapper.find('[data-test="service-new-name"]').setValue(' mesher ')
    createServiceKey.mockResolvedValueOnce({ apiKey: { id: 'k3' }, key: 'tpk_secret' })
    listServices.mockResolvedValue([mesher()])
    await wrapper.find('[data-test="service-new"]').trigger('submit')
    await flushPromises()
    expect(createServiceKey).toHaveBeenCalledWith('mesher')
    expect(wrapper.find('[data-test="service-new-name"]').element.value).toBe('')
    expect(wrapper.find('[data-test="key-created"]').text()).toContain('New API key for mesher')
    expect(wrapper.find('[data-test="key-created-value"]').text()).toBe('tpk_secret')
    expect(wrapper.find('[data-test="service-mesher"]').exists()).toBe(true)

    const writeText = vi.fn().mockResolvedValue()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    await wrapper.find('[data-test="key-created-copy"]').trigger('click')
    expect(writeText).toHaveBeenCalledWith('tpk_secret')

    await wrapper.find('[data-test="key-created-done"]').trigger('click')
    expect(wrapper.find('[data-test="key-created"]').exists()).toBe(false)
  })

  it('adds a key to a service and reports a failure', async () => {
    const wrapper = await mountServices()
    createServiceKey.mockResolvedValueOnce({ apiKey: { id: 'k4' }, key: 'tpk_rotated' })
    await wrapper.find('[data-test="key-create-tmphost"]').trigger('click')
    await flushPromises()
    expect(createServiceKey).toHaveBeenCalledWith('tmphost')
    expect(wrapper.find('[data-test="key-created-value"]').text()).toBe('tpk_rotated')

    const err = new Error('nope')
    createServiceKey.mockRejectedValueOnce(err)
    await wrapper.find('[data-test="key-create-mesher"]').trigger('click')
    await flushPromises()
    expect(wrapper.emitted('error')).toEqual([[err]])
  })

  it('revokes a key after confirming', async () => {
    const wrapper = await mountServices()
    await wrapper.find('[data-test="key-revoke-k2"]').trigger('click')
    expect(wrapper.find('[data-test="delete-modal-message"]').text()).toBe('tpk_new… stops working for mesher at once.')
    await wrapper.find('[data-test="delete-modal-cancel"]').trigger('click')
    expect(wrapper.find('[data-test="delete-modal"]').exists()).toBe(false)

    await wrapper.find('[data-test="key-revoke-k2"]').trigger('click')
    revokeServiceKey.mockResolvedValueOnce()
    await wrapper.find('[data-test="delete-modal-confirm"]').trigger('click')
    await flushPromises()
    expect(revokeServiceKey).toHaveBeenCalledWith('mesher', 'k2')
    expect(toasts()).toContainEqual(['success', 'Revoked tpk_new…'])
    expect(wrapper.find('[data-test="delete-modal"]').exists()).toBe(false)
    expect(listServices).toHaveBeenCalledTimes(2)

    const err = new Error('nope')
    revokeServiceKey.mockRejectedValueOnce(err)
    await wrapper.find('[data-test="key-revoke-k2"]').trigger('click')
    await wrapper.find('[data-test="delete-modal-confirm"]').trigger('click')
    await flushPromises()
    expect(wrapper.emitted('error')).toEqual([[err]])
    expect(wrapper.find('[data-test="delete-modal"]').exists()).toBe(true)
  })
})
