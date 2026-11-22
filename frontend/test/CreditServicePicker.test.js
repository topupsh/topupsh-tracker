// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('../src/api', () => ({ listServices: vi.fn() }))

import CreditServicePicker from '../src/components/CreditServicePicker.vue'
import { listServices } from '../src/api'

function mountPicker(modelValue = '') {
  return mount(CreditServicePicker, {
    props: { modelValue, 'onUpdate:modelValue': (v) => wrapper.setProps({ modelValue: v }) },
  })
}
let wrapper

describe('CreditServicePicker', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('picks the first service, then the one chosen', async () => {
    listServices.mockResolvedValueOnce([{ name: 'mesher' }, { name: 'tmphost' }])
    wrapper = mountPicker()
    await flushPromises()
    expect(wrapper.props('modelValue')).toBe('mesher')
    await wrapper.find('[data-test="service-picker-select"]').setValue('tmphost')
    expect(wrapper.props('modelValue')).toBe('tmphost')
  })

  it('keeps a service that still exists', async () => {
    listServices.mockResolvedValueOnce([{ name: 'mesher' }, { name: 'tmphost' }])
    wrapper = mountPicker('tmphost')
    await flushPromises()
    expect(wrapper.props('modelValue')).toBe('tmphost')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('says when there are no services', async () => {
    listServices.mockResolvedValueOnce([])
    wrapper = mountPicker('gone')
    await flushPromises()
    expect(wrapper.props('modelValue')).toBe('')
    expect(wrapper.find('[data-test="service-picker-empty"]').exists()).toBe(true)
  })

  it('reports a failed load', async () => {
    const err = new Error('nope')
    listServices.mockRejectedValueOnce(err)
    wrapper = mountPicker()
    await flushPromises()
    expect(wrapper.emitted('error')).toEqual([[err]])
  })
})
