// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

function json(body, status = 200) {
  return { ok: status >= 200 && status < 300, status, json: async () => body }
}

async function loadApi(basePath) {
  vi.resetModules()
  window.__TOPUPSH_BASE_PATH__ = basePath
  return import('../src/api.js')
}

describe('api', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })
  afterEach(() => {
    delete window.__TOPUPSH_BASE_PATH__
  })

  it('builds the API base from the injected base path', async () => {
    expect((await loadApi('/app/')).API_BASE_URL).toBe('/app/api/v1')
    expect((await loadApi('/app/')).ADMIN_BASE_URL).toBe('/app/admin/v1')
    expect((await loadApi(undefined)).API_BASE_URL).toBe('/api/v1')
  })

  it('fetchUser caches the user and dedupes concurrent calls', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValue(json({ user: { id: 'u1' } }))
    const [a, b] = await Promise.all([api.fetchUser(), api.fetchUser()])
    expect(a).toEqual({ id: 'u1' })
    expect(b).toBe(a)
    expect(await api.fetchUser()).toBe(a)
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(fetch).toHaveBeenCalledWith('/api/v1/users/me', { credentials: 'same-origin' })

    api.clearUserCache()
    await api.fetchUser()
    expect(fetch).toHaveBeenCalledTimes(2)
  })

  it('fetchUser returns null without a session', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValue(json({ error: 'unauthorized' }, 401))
    expect(await api.fetchUser()).toBeNull()
  })

  it('fetchUser throws on server errors and can retry', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValueOnce(json({}, 500))
    await expect(api.fetchUser()).rejects.toThrow('Failed to load user (500)')
    fetch.mockResolvedValueOnce(json({ user: { id: 'u2' } }))
    expect(await api.fetchUser()).toEqual({ id: 'u2' })
  })

  it('fetchAuthConfig returns providers', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValue(json({ providers: ['google'] }))
    expect(await api.fetchAuthConfig()).toEqual(['google'])
    fetch.mockResolvedValue(json({}, 503))
    await expect(api.fetchAuthConfig()).rejects.toThrow('Failed to load sign-in options (503)')
  })

  it('providerLoginUrl encodes the redirect', async () => {
    const api = await loadApi('')
    expect(api.providerLoginUrl('github')).toBe('/auth/github/login')
    expect(api.providerLoginUrl('google', '/a?b=1')).toBe('/auth/google/login?redirect_url=%2Fa%3Fb%3D1')
  })

  it('logout posts and clears the cache even on failure', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValue(json({ user: { id: 'u1' } }))
    await api.fetchUser()

    fetch.mockResolvedValueOnce(json(null, 204))
    await api.logout()
    expect(fetch).toHaveBeenLastCalledWith('/auth/logout', { method: 'POST', credentials: 'same-origin' })

    fetch.mockResolvedValueOnce(json({ user: { id: 'u1' } }))
    await api.fetchUser()
    fetch.mockRejectedValueOnce(new Error('offline'))
    await expect(api.logout()).rejects.toThrow('offline')
    fetch.mockResolvedValueOnce(json({}, 401))
    expect(await api.fetchUser()).toBeNull()
  })
})

describe('setup api', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  it('setupLogin posts the token and surfaces API errors', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValueOnce(json(null, 204))
    await api.setupLogin('tok')
    expect(fetch).toHaveBeenCalledWith('/api/v1/setup/login', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: '{"token":"tok"}',
    })

    fetch.mockResolvedValueOnce(json({ error: 'invalid root access token' }, 401))
    await expect(api.setupLogin('x')).rejects.toMatchObject({ message: 'invalid root access token', status: 401 })

    fetch.mockResolvedValueOnce({ ok: false, status: 502, json: async () => { throw new Error('html') } })
    await expect(api.setupLogin('x')).rejects.toThrow('Sign-in failed (502)')

    fetch.mockResolvedValueOnce(json({}, 500))
    await expect(api.setupLogin('x')).rejects.toThrow('Sign-in failed (500)')
  })

  it('setupLogout posts', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValueOnce(json(null, 204))
    await api.setupLogout()
    expect(fetch).toHaveBeenCalledWith('/api/v1/setup/logout', { method: 'POST', credentials: 'same-origin' })
  })

  it('fetchSettings returns state, null without a session, throws otherwise', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValueOnce(json({ settings: {}, secretsSet: {}, restartRequired: false }))
    expect(await api.fetchSettings()).toEqual({ settings: {}, secretsSet: {}, restartRequired: false })
    fetch.mockResolvedValueOnce(json({ error: 'unauthorized' }, 401))
    expect(await api.fetchSettings()).toBeNull()
    fetch.mockResolvedValueOnce(json({}, 500))
    await expect(api.fetchSettings()).rejects.toThrow('Failed to load settings (500)')
  })

  it('saveSettings puts and surfaces validation errors', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValueOnce(json({ settings: { tls: { port: 8443 } } }))
    expect(await api.saveSettings({ tls: { port: 8443 } })).toEqual({ settings: { tls: { port: 8443 } } })
    expect(fetch).toHaveBeenCalledWith('/api/v1/setup/settings', {
      method: 'PUT',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: '{"settings":{"tls":{"port":8443}}}',
    })
    fetch.mockResolvedValueOnce(json({ error: 'invalid settings: sqlite DSN is required' }, 400))
    await expect(api.saveSettings({})).rejects.toMatchObject({ message: 'invalid settings: sqlite DSN is required', status: 400 })
  })
})

describe('admin api', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  it('exports the base path', async () => {
    expect((await loadApi('/app/')).BASE_PATH).toBe('/app')
  })

  it('fetchAdminStats unwraps the stats', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValueOnce(json({ stats: { users: 2, days: [] } }))
    expect(await api.fetchAdminStats()).toEqual({ users: 2, days: [] })
    expect(fetch).toHaveBeenCalledWith('/admin/v1/stats', { method: 'GET', credentials: 'same-origin' })
    fetch.mockResolvedValueOnce(json({}, 401))
    await expect(api.fetchAdminStats()).rejects.toMatchObject({ status: 401 })
  })

  it('listAdminUsers searches and pages the users', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValueOnce(json({ users: [], next: '' }))
    expect(await api.listAdminUsers()).toEqual({ users: [], next: '' })
    expect(fetch).toHaveBeenLastCalledWith('/admin/v1/users?limit=50', { method: 'GET', credentials: 'same-origin' })
    fetch.mockResolvedValueOnce(json({ users: [], next: '' }))
    await api.listAdminUsers({ q: 'a@b c', before: 'x' })
    expect(fetch).toHaveBeenLastCalledWith('/admin/v1/users?limit=50&q=a%40b+c&before=x', { method: 'GET', credentials: 'same-origin' })
    fetch.mockResolvedValueOnce(json({}, 500))
    await expect(api.listAdminUsers()).rejects.toThrow('Failed to load users (500)')
  })

  it('fetchAdminUser unwraps one user', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValueOnce(json({ user: { id: 'u1' } }))
    expect(await api.fetchAdminUser('u1')).toEqual({ id: 'u1' })
    expect(fetch).toHaveBeenCalledWith('/admin/v1/users/u1', { method: 'GET', credentials: 'same-origin' })
    fetch.mockResolvedValueOnce(json({ error: 'user not found' }, 404))
    await expect(api.fetchAdminUser('nope')).rejects.toMatchObject({ message: 'user not found', status: 404 })
  })
})
