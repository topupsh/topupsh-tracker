// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('../src/api', () => ({
  fetchAdminStats: vi.fn(),
  fetchAdminUser: vi.fn(),
  listAdminUsers: vi.fn(),
  fetchSettings: vi.fn(),
  setupLogin: vi.fn(),
  setupLogout: vi.fn(),
}))

import {
  fetchAdminStats,
  fetchAdminUser,
  fetchSettings,
  listAdminUsers,
  setupLogin,
  setupLogout,
} from '../src/api'
import AdminView from '../src/views/AdminView.vue'
import { useToasts } from '../src/composables/useToasts'

async function mountView() {
  const wrapper = mount(AdminView)
  await flushPromises()
  return wrapper
}

async function signedIn() {
  fetchSettings.mockResolvedValue({ settings: {} })
  return mountView()
}

const USER = {
  id: 'u1',
  email: 'ann@example.com',
  name: 'Ann',
  picture: '',
  provider: 'github',
  createdAt: '2026-10-01T00:00:00Z',
}

describe('AdminView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    fetchAdminStats.mockResolvedValue({ users: 0, signups: 0, days: [] })
    listAdminUsers.mockResolvedValue({ users: [USER], next: '' })
    fetchAdminUser.mockResolvedValue(USER)
    useToasts().toasts.value = []
  })
  afterEach(() => {
    useToasts().toasts.value = []
  })

  it('shows a spinner while checking the root session', () => {
    fetchSettings.mockReturnValue(new Promise(() => {}))
    expect(mount(AdminView).find('[data-test="admin-loading"]').exists()).toBe(true)
  })

  it('asks for the root token without a session, then opens the admin page', async () => {
    fetchSettings.mockResolvedValueOnce(null).mockResolvedValueOnce({ settings: {} })
    setupLogin.mockResolvedValue()
    const wrapper = await mountView()
    expect(wrapper.find('[data-test="admin-login"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="admin-unlock"]').attributes('disabled')).toBeDefined()

    await wrapper.find('[data-test="admin-token"]').setValue('root-token')
    await wrapper.find('[data-test="admin-login"]').trigger('submit')
    await flushPromises()
    expect(setupLogin).toHaveBeenCalledWith('root-token')
    expect(wrapper.find('[data-test="admin-page"]').exists()).toBe(true)
  })

  it('shows a wrong-token error', async () => {
    fetchSettings.mockResolvedValue(null)
    setupLogin.mockRejectedValue(new Error('invalid root access token'))
    const wrapper = await mountView()
    await wrapper.find('[data-test="admin-token"]').setValue('nope')
    await wrapper.find('[data-test="admin-login"]').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[data-test="admin-login-error"]').text()).toBe('invalid root access token')
  })

  it('falls back to sign-in when the session check fails', async () => {
    fetchSettings.mockRejectedValue(new Error('Failed to load settings (500)'))
    const wrapper = await mountView()
    expect(wrapper.find('[data-test="admin-login"]').exists()).toBe(true)
    expect(useToasts().toasts.value[0].message).toBe('Failed to load settings (500)')
  })

  it('signs out, even when the server call fails', async () => {
    const wrapper = await signedIn()
    setupLogout.mockRejectedValueOnce(new Error('offline'))
    await wrapper.find('[data-test="admin-logout"]').trigger('click')
    await flushPromises()
    expect(setupLogout).toHaveBeenCalled()
    expect(wrapper.find('[data-test="admin-login"]').exists()).toBe(true)
  })

  it('opens on the overview and switches sections', async () => {
    const wrapper = await signedIn()
    expect(wrapper.find('[data-test="admin-overview"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="admin-tab-overview"]').attributes('aria-current')).toBe('page')
    expect(fetchAdminStats).toHaveBeenCalled()

    await wrapper.find('[data-test="admin-tab-users"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="admin-users"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="admin-tab-overview"]').attributes('aria-current')).toBeUndefined()
  })

  it('opens a user, goes back to the list, and resets on a section switch', async () => {
    const wrapper = await signedIn()
    await wrapper.find('[data-test="admin-tab-users"]').trigger('click')
    await flushPromises()
    await wrapper.find('[data-test="admin-user-u1"]').trigger('click')
    await flushPromises()
    expect(fetchAdminUser).toHaveBeenCalledWith('u1')
    expect(wrapper.find('[data-test="admin-user-name"]').text()).toBe('Ann')

    await wrapper.find('[data-test="admin-user-back"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="admin-users"]').exists()).toBe(true)

    await wrapper.find('[data-test="admin-user-u1"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="admin-user"]').exists()).toBe(true)

    // Opening the users section again starts at the list.
    await wrapper.find('[data-test="admin-tab-users"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="admin-users"]').exists()).toBe(true)
  })

  it('reports section errors, and asks to sign in again on 401', async () => {
    fetchAdminStats.mockRejectedValueOnce(new Error('Failed to load stats (500)'))
    const wrapper = await signedIn()
    expect(useToasts().toasts.value[0].message).toBe('Failed to load stats (500)')
    expect(wrapper.find('[data-test="admin-page"]').exists()).toBe(true)

    listAdminUsers.mockRejectedValueOnce(Object.assign(new Error('unauthorized'), { status: 401 }))
    await wrapper.find('[data-test="admin-tab-users"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="admin-login"]').exists()).toBe(true)
  })

  it('signs out back to the overview', async () => {
    const wrapper = await signedIn()
    await wrapper.find('[data-test="admin-tab-users"]').trigger('click')
    await wrapper.find('[data-test="admin-logout"]').trigger('click')
    await flushPromises()
    fetchSettings.mockResolvedValue({ settings: {} })
    setupLogin.mockResolvedValue()
    await wrapper.find('[data-test="admin-token"]').setValue('root-token')
    await wrapper.find('[data-test="admin-login"]').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[data-test="admin-overview"]').exists()).toBe(true)
  })
})
