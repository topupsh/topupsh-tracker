// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('../src/api', () => ({ fetchAdminUser: vi.fn() }))

import AdminUserDetail from '../src/components/AdminUserDetail.vue'
import { fetchAdminUser } from '../src/api'

const USER = {
  id: 'u1',
  email: 'ann@example.com',
  name: 'Ann',
  picture: 'https://example.com/a.png',
  provider: 'github',
  createdAt: '2026-10-01T00:00:00Z',
}

const mountDetail = () => mount(AdminUserDetail, { props: { id: 'u1' } })

describe('AdminUserDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows the user and goes back', async () => {
    fetchAdminUser.mockResolvedValue(USER)
    const wrapper = mountDetail()
    expect(wrapper.find('[data-test="admin-user-loading"]').exists()).toBe(true)
    await flushPromises()

    expect(fetchAdminUser).toHaveBeenCalledWith('u1')
    expect(wrapper.find('[data-test="admin-user-name"]').text()).toBe('Ann')
    expect(wrapper.find('[data-test="admin-user-id"]').text()).toBe('u1')
    expect(wrapper.text()).toContain('github')
    expect(wrapper.find('img').attributes('src')).toBe(USER.picture)

    await wrapper.find('[data-test="admin-user-back"]').trigger('click')
    expect(wrapper.emitted('back')).toHaveLength(1)
  })

  it('falls back to the email without a name or picture', async () => {
    fetchAdminUser.mockResolvedValue({ ...USER, name: '', picture: '', provider: '' })
    const wrapper = mountDetail()
    await flushPromises()
    expect(wrapper.find('[data-test="admin-user-name"]').text()).toBe('ann@example.com')
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('github')
  })

  it('says when no user has the ID', async () => {
    fetchAdminUser.mockRejectedValue(Object.assign(new Error('user not found'), { status: 404 }))
    const wrapper = mountDetail()
    await flushPromises()
    expect(wrapper.find('[data-test="admin-user-missing"]').text()).toContain('u1')
    expect(wrapper.emitted('error')).toBeUndefined()
  })

  it('reports other failures', async () => {
    const err = new Error('Failed to load user (500)')
    fetchAdminUser.mockRejectedValue(err)
    const wrapper = mountDetail()
    await flushPromises()
    expect(wrapper.find('[data-test="admin-user-failed"]').exists()).toBe(true)
    expect(wrapper.emitted('error')).toEqual([[err]])
  })
})
