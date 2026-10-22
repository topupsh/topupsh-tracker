// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, expect, it, vi } from 'vitest'

const mount = vi.fn()
const createTrackerApp = vi.fn(() => ({ app: { mount } }))
vi.mock('../src/app', () => ({ createTrackerApp }))
const registerServiceWorker = vi.fn()
vi.mock('../src/composables/usePwaUpdate', () => ({ registerServiceWorker }))

describe('main', () => {
  it('boots the web app under the injected base path', async () => {
    window.__TOPUPSH_BASE_PATH__ = '/base'
    await import('../src/main.js')
    expect(createTrackerApp).toHaveBeenCalledWith({ history: expect.anything(), platform: 'web' })
    expect(createTrackerApp.mock.calls[0][0].history.base).toBe('/base')
    expect(mount).toHaveBeenCalledWith('#app')
    expect(registerServiceWorker).toHaveBeenCalledWith({ basePath: '/base' })
    delete window.__TOPUPSH_BASE_PATH__
  })

  it('defaults the base path to /', async () => {
    vi.resetModules()
    createTrackerApp.mockClear()
    registerServiceWorker.mockClear()
    await import('../src/main.js')
    expect(createTrackerApp.mock.calls[0][0].history.base).toBe('')
    expect(registerServiceWorker).toHaveBeenCalledWith({ basePath: '' })
  })
})
