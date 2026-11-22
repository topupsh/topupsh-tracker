// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('../src/api', () => ({ listOrgUsage: vi.fn(), setOrgLimits: vi.fn() }))

import CreditOrgs from '../src/components/CreditOrgs.vue'
import { listOrgUsage, setOrgLimits } from '../src/api'
import { useToasts } from '../src/composables/useToasts'

const ORGS = [
  { org: 'acme', orgLimit: 1000, userLimit: 300, custom: true, used: 425 },
  { org: 'initech', orgLimit: 0, userLimit: null, custom: true, used: 0 },
  { org: 'umbrella', orgLimit: 200, userLimit: 50, custom: false, used: 250 },
]

async function mountOrgs(orgs = ORGS) {
  listOrgUsage.mockResolvedValue(orgs)
  const wrapper = mount(CreditOrgs, { props: { service: 'mesher' } })
  await flushPromises()
  return wrapper
}

const bar = (wrapper, org) => wrapper.find(`[data-test="org-${org}"] [style]`)

describe('CreditOrgs', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useToasts().toasts.value = []
  })

  it('lists orgs with their limits and usage', async () => {
    listOrgUsage.mockReturnValueOnce(new Promise(() => {}))
    expect(mount(CreditOrgs, { props: { service: 'mesher' } }).find('[data-test="orgs-loading"]').exists()).toBe(true)

    const wrapper = await mountOrgs()
    expect(listOrgUsage).toHaveBeenCalledWith('mesher')
    const acme = wrapper.find('[data-test="org-acme"]')
    expect(acme.text()).toContain('1,000')
    expect(acme.text()).toContain('300')
    expect(acme.text()).toContain('425')
    expect(acme.text()).not.toContain('Default')
    expect(bar(wrapper, 'acme').attributes('style')).toContain('width: 43%')
    expect(wrapper.find('[data-test="org-initech"]').text()).toContain('Uncapped')
    expect(bar(wrapper, 'initech').attributes('style')).toContain('width: 100%')
    expect(bar(wrapper, 'initech').classes()).toContain('bg-rose-500')
    expect(wrapper.find('[data-test="org-umbrella"]').text()).toContain('Default')
    expect(bar(wrapper, 'umbrella').attributes('style')).toContain('width: 100%')
  })

  it('says when there are no orgs and reports a failed load', async () => {
    const wrapper = await mountOrgs([])
    expect(wrapper.find('[data-test="orgs-empty"]').exists()).toBe(true)

    const err = new Error('boom')
    listOrgUsage.mockRejectedValueOnce(err)
    const failed = mount(CreditOrgs, { props: { service: 'mesher' } })
    await flushPromises()
    expect(failed.emitted('error')).toEqual([[err]])
  })

  it('edits an org from its row and saves the limits', async () => {
    const wrapper = await mountOrgs()
    await wrapper.find('[data-test="org-initech"]').trigger('click')
    expect(wrapper.find('[data-test="org-limits-org"]').element.value).toBe('initech')
    expect(wrapper.find('[data-test="org-limits-org-limit"]').element.value).toBe('0')
    expect(wrapper.find('[data-test="org-limits-user-limit"]').element.value).toBe('')

    await wrapper.find('[data-test="org-limits-org-limit"]').setValue('500')
    await wrapper.find('[data-test="org-limits-user-limit"]').setValue('20')
    setOrgLimits.mockResolvedValueOnce({})
    await wrapper.find('[data-test="org-limits-form"]').trigger('submit')
    await flushPromises()
    expect(setOrgLimits).toHaveBeenCalledWith('mesher', 'initech', { orgLimit: 500, userLimit: 20 })
    expect(useToasts().toasts.value.at(-1).message).toBe('Saved the limits of initech')
    expect(wrapper.find('[data-test="org-limits-org"]').element.value).toBe('')
    expect(listOrgUsage).toHaveBeenCalledTimes(2)
  })

  it('checks the limits and reports a failed save', async () => {
    const wrapper = await mountOrgs()
    await wrapper.find('[data-test="org-limits-org"]').setValue('new')
    for (const [org, user] of [['', ''], ['1.5', ''], ['1', 'x']]) {
      await wrapper.find('[data-test="org-limits-org-limit"]').setValue(org)
      await wrapper.find('[data-test="org-limits-user-limit"]').setValue(user)
      await wrapper.find('[data-test="org-limits-form"]').trigger('submit')
    }
    expect(setOrgLimits).not.toHaveBeenCalled()
    expect(useToasts().toasts.value.filter((t) => t.type === 'error')).toHaveLength(3)

    await wrapper.find('[data-test="org-limits-org-limit"]').setValue('5')
    await wrapper.find('[data-test="org-limits-user-limit"]').setValue('')
    const err = new Error('nope')
    setOrgLimits.mockRejectedValueOnce(err)
    await wrapper.find('[data-test="org-limits-form"]').trigger('submit')
    await flushPromises()
    expect(setOrgLimits).toHaveBeenCalledWith('mesher', 'new', { orgLimit: 5, userLimit: null })
    expect(wrapper.emitted('error')).toEqual([[err]])
    expect(wrapper.find('[data-test="org-limits-org"]').element.value).toBe('new')
  })
})
