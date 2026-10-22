// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import HomeView from '../src/views/HomeView.vue'

function mountHome(user) {
  return mount(HomeView, { global: { provide: { currentUser: { user: ref(user), refreshUser: async () => {} } } } })
}

describe('HomeView', () => {
  it('greets the user by name, then email, and shows the empty state', () => {
    expect(mountHome({ name: 'Ada', email: 'ada@x' }).find('[data-test="home-greeting"]').text()).toBe('Welcome, Ada')
    expect(mountHome({ name: '', email: 'ada@x' }).find('[data-test="home-greeting"]').text()).toBe('Welcome, ada@x')
    const anonymous = mountHome(null)
    expect(anonymous.find('[data-test="home-greeting"]').text()).toBe('Welcome')
    expect(anonymous.find('[data-test="home-empty"]').text()).toContain('Nothing here yet')
  })
})
