// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia } from 'pinia'
import AppSidebar from '../src/components/AppSidebar.vue'
import { clickOutside } from '../src/app'
import { useTooltipStore } from '../src/stores/tooltipStore'

const user = {
  id: 'u1',
  name: 'Ada Lovelace',
  email: 'ada@example.com',
  picture: '',
}

async function mountSidebar(props = {}, path = '/') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { render: () => null } }, { path: '/other', component: { render: () => null } }],
  })
  await router.push(path)
  return mount(AppSidebar, {
    props: { user, ...props },
    attachTo: document.body,
    global: { plugins: [router, createPinia()], directives: { 'click-outside': clickOutside } },
  })
}

describe('AppSidebar', () => {
  it('shows brand, nav and the user', async () => {
    const wrapper = await mountSidebar()
    expect(wrapper.find('[data-test="sidebar-brand"]').text()).toBe('topupsh tracker')
    expect(wrapper.findAll('a').find((a) => a.text() === 'Home').attributes('href')).toBe('/')
    expect(wrapper.text()).toContain('Ada Lovelace')
    expect(wrapper.find('[data-test="user-menu-toggle"]').text()).toContain('A')
    expect(wrapper.find('a[href="/"].font-semibold').exists()).toBe(true)
    wrapper.unmount()

    const elsewhere = await mountSidebar({}, '/other')
    expect(elsewhere.findAll('a').find((a) => a.text() === 'Home').classes()).not.toContain('font-semibold')
    elsewhere.unmount()
  })

  it('hides labels when collapsed and shows them in the mobile drawer', async () => {
    const collapsed = await mountSidebar({ collapsed: true })
    expect(collapsed.find('[data-test="sidebar-brand"]').exists()).toBe(false)
    expect(collapsed.find('[data-test="sidebar-toggle"]').attributes('title')).toBe('Expand sidebar')
    expect(collapsed.find('a[aria-label="Home"]').exists()).toBe(true)
    collapsed.unmount()

    const drawer = await mountSidebar({ collapsed: true, mobileOpen: true })
    expect(drawer.find('[data-test="sidebar-brand"]').exists()).toBe(true)
    expect(drawer.find('[data-test="sidebar"]').classes()).toContain('w-[280px]')
    drawer.unmount()
  })

  it('names the items in a tooltip only while collapsed', async () => {
    const collapsed = await mountSidebar({ collapsed: true })
    const tooltip = useTooltipStore()
    const home = collapsed.find('a[aria-label="Home"]')
    await home.trigger('mouseenter')
    expect(tooltip.visible).toBe(true)
    expect(tooltip.text).toBe('Home')
    await home.trigger('mouseleave')
    expect(tooltip.visible).toBe(false)

    const account = collapsed.find('[data-test="user-menu-toggle"]')
    await account.trigger('mouseenter')
    expect(tooltip.text).toBe('Ada Lovelace')
    await account.trigger('click')
    expect(tooltip.visible).toBe(false)

    // Navigating or expanding drops a tooltip left on screen.
    await home.trigger('mouseenter')
    await collapsed.vm.$router.push('/other')
    await flushPromises()
    expect(tooltip.visible).toBe(false)
    await home.trigger('mouseenter')
    await collapsed.setProps({ collapsed: false })
    expect(tooltip.visible).toBe(false)

    // Labels are visible when expanded, so no tooltip.
    await collapsed.findAll('a').find((a) => a.text() === 'Home').trigger('mouseenter')
    expect(tooltip.visible).toBe(false)
    expect(collapsed.find('a[aria-label]').exists()).toBe(false)
    collapsed.unmount()
  })

  it('emits toggle-collapse', async () => {
    const wrapper = await mountSidebar()
    expect(wrapper.find('[data-test="sidebar-toggle"]').attributes('title')).toBe('Collapse sidebar')
    await wrapper.find('[data-test="sidebar-toggle"]').trigger('click')
    expect(wrapper.emitted('toggle-collapse')).toHaveLength(1)
    wrapper.unmount()
  })

  it('opens the user menu, closes it on outside click, and logs out', async () => {
    const wrapper = await mountSidebar()
    await wrapper.find('[data-test="user-menu-toggle"]').trigger('click')
    const menu = wrapper.find('[data-test="user-menu"]')
    expect(menu.text()).toContain('ada@example.com')

    document.body.click()
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-test="user-menu"]').exists()).toBe(false)

    await wrapper.find('[data-test="user-menu-toggle"]').trigger('click')
    await wrapper.find('[data-test="logout"]').trigger('click')
    expect(wrapper.emitted('logout')).toHaveLength(1)
    expect(wrapper.find('[data-test="user-menu"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('switches the theme from the user menu', async () => {
    localStorage.clear()
    const wrapper = await mountSidebar()
    await wrapper.find('[data-test="user-menu-toggle"]').trigger('click')
    expect(wrapper.find('[data-test="theme-system"]').attributes('aria-checked')).toBe('true')

    await wrapper.find('[data-test="theme-dark"]').trigger('click')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')
    expect(wrapper.find('[data-test="theme-dark"]').attributes('aria-checked')).toBe('true')
    expect(wrapper.find('[data-test="theme-system"]').attributes('aria-checked')).toBe('false')
    // Picking a theme keeps the menu open.
    expect(wrapper.find('[data-test="user-menu"]').exists()).toBe(true)

    await wrapper.find('[data-test="theme-light"]').trigger('click')
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    localStorage.clear()
    wrapper.unmount()
  })

  it('positions the menu beside a collapsed sidebar', async () => {
    const wrapper = await mountSidebar({ collapsed: true })
    await wrapper.find('[data-test="user-menu-toggle"]').trigger('click')
    expect(wrapper.find('[data-test="user-menu"]').classes()).toContain('left-full')
    wrapper.unmount()
  })

  it('handles a picture, an email-only user and no user', async () => {
    const withPicture = await mountSidebar({ user: { ...user, picture: 'https://img/p.png' } })
    expect(withPicture.find('img').attributes('src')).toBe('https://img/p.png')
    withPicture.unmount()

    const emailOnly = await mountSidebar({ user: { ...user, name: '' } })
    expect(emailOnly.find('[data-test="user-menu-toggle"]').text()).toContain('ada@example.com')
    await emailOnly.find('[data-test="user-menu-toggle"]').trigger('click')
    // The email is the display name, so it is not repeated under it.
    expect(emailOnly.find('[data-test="user-menu"]').text().split('ada@example.com')).toHaveLength(2)
    emailOnly.unmount()

    const none = await mountSidebar({ user: null })
    expect(none.find('[data-test="user-menu-toggle"]').text()).toContain('Account')
    none.unmount()
  })
})
