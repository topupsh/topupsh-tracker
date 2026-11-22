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

describe('credit admin api', () => {
  const GET = { method: 'GET', credentials: 'same-origin' }
  const withBody = (method, body) => ({
    method,
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })

  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  it('listServices unwraps the services', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValueOnce(json({ services: [{ name: 'mesher' }] }))
    expect(await api.listServices()).toEqual([{ name: 'mesher' }])
    expect(fetch).toHaveBeenCalledWith('/api/v1/services', GET)
  })

  it('creates and revokes service keys', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValueOnce(json({ apiKey: { id: 'k1' }, key: 'tpk_x' }, 201))
    expect(await api.createServiceKey('my svc')).toEqual({ apiKey: { id: 'k1' }, key: 'tpk_x' })
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/services/my%20svc/keys', { method: 'POST', credentials: 'same-origin' })

    fetch.mockResolvedValueOnce({ ok: true, status: 204, json: async () => null })
    await api.revokeServiceKey('mesher', 'k1')
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/services/mesher/keys/k1', { method: 'DELETE', credentials: 'same-origin' })

    fetch.mockResolvedValueOnce(json({ error: 'key not found' }, 404))
    await expect(api.revokeServiceKey('mesher', 'k2')).rejects.toMatchObject({ message: 'key not found', status: 404 })
    fetch.mockResolvedValueOnce(json({}, 500))
    await expect(api.createServiceKey('mesher')).rejects.toThrow('Failed to create API key (500)')
  })

  it('sets the switch, the defaults and org limits', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValue(json({ name: 'mesher' }))
    await api.setServiceEnabled('mesher', true)
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/services/mesher/enabled', withBody('PUT', { enabled: true }))
    await api.setServiceDefaults('mesher', { orgLimit: 10, userLimit: null })
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/services/mesher/defaults', withBody('PUT', { orgLimit: 10, userLimit: null }))
    await api.setOrgLimits('mesher', 'o/1', { orgLimit: 5, userLimit: 2 })
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/orgs/o%2F1/limits', withBody('PUT', { service: 'mesher', orgLimit: 5, userLimit: 2 }))
  })

  it('lists org and user usage', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValueOnce(json({ orgs: [{ org: 'o1' }] }))
    expect(await api.listOrgUsage('mesher')).toEqual([{ org: 'o1' }])
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/services/mesher/orgs', GET)

    fetch.mockResolvedValue(json({ users: [], truncated: false }))
    expect(await api.listUserUsage('mesher')).toEqual({ users: [], truncated: false })
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/services/mesher/users', GET)
    await api.listUserUsage('mesher', 'o 1')
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/services/mesher/users?org=o+1', GET)
  })

  it('creates a grant', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValueOnce(json({ id: 'g1' }, 201))
    const grant = { service: 'mesher', org: 'o1', user: 'u1', credits: 5 }
    expect(await api.createGrant(grant)).toEqual({ id: 'g1' })
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/grants', withBody('POST', grant))
  })

  it('listLedger filters and pages the ledger', async () => {
    const api = await loadApi('')
    fetch.mockResolvedValue(json({ entries: [] }))
    expect(await api.listLedger()).toEqual({ entries: [] })
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/ledger?limit=50', GET)
    await api.listLedger({ service: 'mesher', org: 'o1', user: 'u1', cursor: 'c' })
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/ledger?limit=50&service=mesher&org=o1&user=u1&cursor=c', GET)
  })
})

describe('viewer api', () => {
  beforeEach(() => {
    globalThis.fetch = vi.fn()
  })

  it('reads the balance and pages the history with the view token', async () => {
    const api = await loadApi('')
    const auth = { headers: { Authorization: 'Bearer tok' } }
    fetch.mockResolvedValueOnce(json({ balance: 5 }))
    expect(await api.fetchViewerBalance('tok')).toEqual({ balance: 5 })
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/balances', auth)

    fetch.mockResolvedValue(json({ entries: [] }))
    await api.listViewerLedger('tok')
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/ledger?limit=50', auth)
    await api.listViewerLedger('tok', 'c1')
    expect(fetch).toHaveBeenLastCalledWith('/api/v1/ledger?limit=50&cursor=c1', auth)

    fetch.mockResolvedValueOnce(json({ error: 'invalid token' }, 401))
    await expect(api.fetchViewerBalance('old')).rejects.toMatchObject({ message: 'invalid token', status: 401 })
  })
})
