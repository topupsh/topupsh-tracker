// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { usePlatformStore } from '../src/stores/platformStore'
import { useToasts } from '../src/composables/useToasts'
import { CHART_COLORS, HISTORY_DAYS } from '../src/utils/stats'
import { loginErrorMessage } from '../src/utils/loginErrors'
import { useClipboard } from '../src/composables/useClipboard'

describe('platformStore', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('accepts known platforms only', () => {
    const store = usePlatformStore()
    expect(store.isWeb).toBe(true)
    store.setPlatform('desktop')
    expect(store.isDesktop).toBe(true)
    expect(store.isWeb).toBe(false)
    store.setPlatform('toaster')
    expect(store.platform).toBe('web')
  })
})

describe('useToasts', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => {
    useToasts().toasts.value = []
    vi.useRealTimers()
  })

  it('adds, expires and caps toasts', () => {
    const { toasts, notifyInfo, notifySuccess, notifyError, removeToast } = useToasts()
    const id = notifyInfo('hello', 'Title')
    expect(toasts.value[0]).toMatchObject({ id, message: 'hello', type: 'info', title: 'Title' })
    notifySuccess('ok')
    notifyError('bad')
    expect(toasts.value.map((t) => t.type)).toEqual(['info', 'success', 'error'])

    vi.advanceTimersByTime(4000)
    expect(toasts.value.map((t) => t.type)).toEqual(['error'])
    vi.advanceTimersByTime(4000)
    expect(toasts.value).toEqual([])

    for (let i = 0; i < 6; i++) notifyInfo(`m${i}`)
    expect(toasts.value.map((t) => t.message)).toEqual(['m2', 'm3', 'm4', 'm5'])
    removeToast(toasts.value[0].id)
    expect(toasts.value).toHaveLength(3)
  })

  it('addToast defaults to info', () => {
    const { toasts, addToast } = useToasts()
    addToast('plain')
    expect(toasts.value[0]).toMatchObject({ type: 'info', title: null })
  })
})

describe('stats', () => {
  it('names the admin chart colours and range', () => {
    expect(CHART_COLORS.signups).toMatch(/^#[0-9a-f]{6}$/)
    expect(HISTORY_DAYS).toBe(30)
  })
})

describe('loginErrorMessage', () => {
  it('maps known codes and falls back', () => {
    expect(loginErrorMessage(undefined)).toBe('')
    expect(loginErrorMessage('cancelled')).toBe('Sign-in was cancelled.')
    expect(loginErrorMessage('email_not_verified')).toMatch(/verified email/)
    expect(loginErrorMessage('weird')).toMatch(/Something went wrong/)
  })
})

describe('providers', () => {
  it('slugs names like the server', async () => {
    const { providerSlug } = await import('../src/utils/providers')
    expect(providerSlug(' My  Okta! ')).toBe('my-okta')
    expect(providerSlug('--A_b--')).toBe('a-b')
    expect(providerSlug('')).toBe('')
    expect(providerSlug(undefined)).toBe('')
  })

  it('renders SVG as a data URI', async () => {
    const { svgDataUri } = await import('../src/utils/providers')
    expect(svgDataUri('<svg a="#"/>')).toBe('data:image/svg+xml;charset=utf-8,%3Csvg%20a%3D%22%23%22%2F%3E')
  })
})

describe('useClipboard', () => {
  afterEach(() => {
    useToasts().toasts.value = []
  })

  it('copies and toasts success or failure', async () => {
    const writeText = vi.fn().mockResolvedValueOnce().mockRejectedValueOnce(new Error('denied'))
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const { copy } = useClipboard()
    expect(await copy('hello')).toBe(true)
    expect(writeText).toHaveBeenCalledWith('hello')
    expect(useToasts().toasts.value.at(-1)).toMatchObject({ type: 'success', message: 'Copied to clipboard' })
    expect(await copy('x', 'Link copied')).toBe(false)
    expect(useToasts().toasts.value.at(-1)).toMatchObject({ type: 'error', message: 'Could not copy to the clipboard' })
    delete navigator.clipboard
  })
})
