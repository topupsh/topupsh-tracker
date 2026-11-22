// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// The Go server injects the base path the app is served under.
export const BASE_PATH = (window.__TOPUPSH_BASE_PATH__ || '').replace(/\/$/, '')

export const API_BASE_URL = `${BASE_PATH}/api/v1`

// ADMIN_BASE_URL is the root-only admin API, apart from the public one.
export const ADMIN_BASE_URL = `${BASE_PATH}/admin/v1`

let userCache = null
let userPromise = null

// fetchUser returns the signed-in user, or null when there is no session.
// The result is cached until clearUserCache() so route changes don't refetch.
export function fetchUser() {
  if (userCache) return Promise.resolve(userCache)
  if (userPromise) return userPromise
  userPromise = (async () => {
    try {
      const res = await fetch(`${API_BASE_URL}/users/me`, { credentials: 'same-origin' })
      if (res.status === 401) return null
      if (!res.ok) throw new Error(`Failed to load user (${res.status})`)
      const body = await res.json()
      userCache = body.user
      return userCache
    } finally {
      userPromise = null
    }
  })()
  return userPromise
}

export function clearUserCache() {
  userCache = null
}

// fetchAuthConfig lists the sign-in providers the server has enabled.
export async function fetchAuthConfig() {
  const res = await fetch(`${BASE_PATH}/auth/config`)
  if (!res.ok) throw new Error(`Failed to load sign-in options (${res.status})`)
  const body = await res.json()
  return body.providers
}

// providerLoginUrl is the server route that starts an OAuth round trip.
// redirect is an in-app path to land on afterwards.
export function providerLoginUrl(provider, redirect) {
  const url = `${BASE_PATH}/auth/${encodeURIComponent(provider)}/login`
  return redirect ? `${url}?redirect_url=${encodeURIComponent(redirect)}` : url
}

export async function logout() {
  try {
    await fetch(`${BASE_PATH}/auth/logout`, { method: 'POST', credentials: 'same-origin' })
  } finally {
    clearUserCache()
  }
}

// httpError builds an Error from an API error body, keeping the status so
// callers can tell an expired session (401) from other failures.
async function httpError(res, fallback) {
  let message = fallback
  try {
    const body = await res.json()
    if (body && body.error) message = body.error
  } catch {
    // Not JSON; use the fallback.
  }
  const err = new Error(message)
  err.status = res.status
  return err
}

// setupLogin exchanges the root access token for a root session cookie.
export async function setupLogin(token) {
  const res = await fetch(`${API_BASE_URL}/setup/login`, {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token }),
  })
  if (!res.ok) throw await httpError(res, `Sign-in failed (${res.status})`)
}

export async function setupLogout() {
  await fetch(`${API_BASE_URL}/setup/logout`, { method: 'POST', credentials: 'same-origin' })
}

// fetchSettings returns { settings, secretsSet, restartRequired }, or null
// without a root session.
export async function fetchSettings() {
  const res = await fetch(`${API_BASE_URL}/setup/settings`, { credentials: 'same-origin' })
  if (res.status === 401) return null
  if (!res.ok) throw await httpError(res, `Failed to load settings (${res.status})`)
  return res.json()
}

// saveSettings replaces the settings. Blank secrets keep their stored value.
export async function saveSettings(settings) {
  const res = await fetch(`${API_BASE_URL}/setup/settings`, {
    method: 'PUT',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ settings }),
  })
  if (!res.ok) throw await httpError(res, `Failed to save settings (${res.status})`)
  return res.json()
}

// request sends a GET for root (base ADMIN_BASE_URL) or the signed-in user
// and returns the parsed body. Failures throw an Error carrying the HTTP
// status.
async function request(path, { fallback, base = API_BASE_URL }) {
  const res = await fetch(`${base}${path}`, { method: 'GET', credentials: 'same-origin' })
  if (!res.ok) throw await httpError(res, `${fallback} (${res.status})`)
  return res.json()
}

// fetchAdminStats returns root's stats over every account: { users, signups,
// days: [{ date, signups }] }.
export async function fetchAdminStats() {
  const { stats } = await request('/stats', { fallback: 'Failed to load stats', base: ADMIN_BASE_URL })
  return stats
}

// listAdminUsers returns a page of users, newest first, as { users, next }:
// q searches emails and names, or names one user ID; pass next back as
// before for the next page.
export function listAdminUsers({ q = '', before = '' } = {}) {
  const query = new URLSearchParams({ limit: String(ADMIN_USERS_PAGE_SIZE) })
  if (q) query.set('q', q)
  if (before) query.set('before', before)
  return request(`/users?${query}`, { fallback: 'Failed to load users', base: ADMIN_BASE_URL })
}

// ADMIN_USERS_PAGE_SIZE is how many users a page of the admin list shows.
export const ADMIN_USERS_PAGE_SIZE = 50

// fetchAdminUser returns one user, as root.
export async function fetchAdminUser(id) {
  const { user } = await request(`/users/${encodeURIComponent(id)}`, { fallback: 'Failed to load user', base: ADMIN_BASE_URL })
  return user
}

// send makes a root call with a JSON body (when given) and returns the
// parsed body, or null for 204. Failures throw an Error carrying the HTTP
// status.
async function send(method, path, { body, fallback }) {
  const init = { method, credentials: 'same-origin' }
  if (body !== undefined) {
    init.headers = { 'Content-Type': 'application/json' }
    init.body = JSON.stringify(body)
  }
  const res = await fetch(`${API_BASE_URL}${path}`, init)
  if (!res.ok) throw await httpError(res, `${fallback} (${res.status})`)
  return res.status === 204 ? null : res.json()
}

function servicePath(service) {
  return `/services/${encodeURIComponent(service)}`
}

// listServices returns every credit service, with its switch, its default
// limits and its API keys, newest first.
export async function listServices() {
  const { services } = await request('/services', { fallback: 'Failed to load services' })
  return services
}

// createServiceKey issues an API key, creating the service when it is new.
// The returned { apiKey, key } is the only time the key itself is shown.
export function createServiceKey(service) {
  return send('POST', `${servicePath(service)}/keys`, { fallback: 'Failed to create API key' })
}

export async function revokeServiceKey(service, id) {
  await send('DELETE', `${servicePath(service)}/keys/${encodeURIComponent(id)}`, { fallback: 'Failed to revoke API key' })
}

// setServiceEnabled switches credit limiting on or off for a service.
export function setServiceEnabled(service, enabled) {
  return send('PUT', `${servicePath(service)}/enabled`, { body: { enabled }, fallback: 'Failed to update service' })
}

// setServiceDefaults sets the limits of the orgs without their own; a null
// userLimit leaves users uncapped.
export function setServiceDefaults(service, { orgLimit, userLimit }) {
  return send('PUT', `${servicePath(service)}/defaults`, { body: { orgLimit, userLimit }, fallback: 'Failed to save defaults' })
}

// listOrgUsage returns a service's orgs with their limits and this month's
// usage.
export async function listOrgUsage(service) {
  const { orgs } = await request(`${servicePath(service)}/orgs`, { fallback: 'Failed to load orgs' })
  return orgs
}

// setOrgLimits sets an org's limits; a null userLimit leaves users uncapped.
export function setOrgLimits(service, org, { orgLimit, userLimit }) {
  return send('PUT', `/orgs/${encodeURIComponent(org)}/limits`, {
    body: { service, orgLimit, userLimit },
    fallback: 'Failed to save limits',
  })
}

// listUserUsage returns { users, truncated }: a service's users with credits,
// of one org when org is set.
export function listUserUsage(service, org = '') {
  const query = org ? `?${new URLSearchParams({ org })}` : ''
  return request(`${servicePath(service)}/users${query}`, { fallback: 'Failed to load usage' })
}

// createGrant gives a user extra credits that expire in a month.
export function createGrant({ service, org, user, credits }) {
  return send('POST', '/grants', { body: { service, org, user, credits }, fallback: 'Failed to grant credits' })
}

// LEDGER_PAGE_SIZE is how many entries a page of the admin ledger shows.
export const LEDGER_PAGE_SIZE = 50

// listLedger returns a page of the ledger, newest first, as
// { entries, nextCursor }; empty filters match everything.
export function listLedger({ service = '', org = '', user = '', cursor = '' } = {}) {
  const query = new URLSearchParams({ limit: String(LEDGER_PAGE_SIZE) })
  for (const [k, v] of Object.entries({ service, org, user, cursor })) {
    if (v) query.set(k, v)
  }
  return request(`/ledger?${query}`, { fallback: 'Failed to load ledger' })
}

// viewerRequest reads as the user a service-issued view token names: the
// token alone says which service, org and user. Failures throw an Error
// carrying the HTTP status (401 once the token has expired).
async function viewerRequest(path, token, fallback) {
  const res = await fetch(`${API_BASE_URL}${path}`, { headers: { Authorization: `Bearer ${token}` } })
  if (!res.ok) throw await httpError(res, `${fallback} (${res.status})`)
  return res.json()
}

// fetchViewerBalance returns the view token's user's balance: { service,
// balance, available, monthly, orgUsage, grants, renewsAt, limited }.
export function fetchViewerBalance(token) {
  return viewerRequest('/balances', token, 'Failed to load your balance')
}

// listViewerLedger returns a page of the view token's user's history,
// newest first, as { entries, nextCursor }.
export function listViewerLedger(token, cursor = '') {
  const query = new URLSearchParams({ limit: String(LEDGER_PAGE_SIZE) })
  if (cursor) query.set('cursor', cursor)
  return viewerRequest(`/ledger?${query}`, token, 'Failed to load your history')
}
