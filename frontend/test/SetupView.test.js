// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

vi.mock('../src/api', () => ({
  fetchSettings: vi.fn(),
  saveSettings: vi.fn(),
  setupLogin: vi.fn(),
  setupLogout: vi.fn(),
}))

import { fetchSettings, saveSettings, setupLogin, setupLogout } from '../src/api'
import SetupView from '../src/views/SetupView.vue'
import { useToasts } from '../src/composables/useToasts'

function state(overrides = {}) {
  return {
    settings: {
      domain: { appUrl: 'https://tracker.topup.sh/', allowedOrigins: ['https://topup.sh'] },
      tls: { enabled: false, port: 443, email: '' },
      database: {
        driver: 'sqlite',
        sqlite: { dsn: './_storage/topupsh.db' },
        postgres: { dsn: '' },
      },
      auth: {
        google: { clientId: 'g', clientSecret: '' },
        github: { clientId: '', clientSecret: '' },
        providers: [{ id: 'my-okta', name: 'My Okta', icon: '<svg></svg>', clientId: 'oc', clientSecret: '', authUrl: 'https://okta.example.com' }],
      },
    },
    secretsSet: {
      postgresDsn: true,
      googleClientSecret: true,
      githubClientSecret: false,
      providerClientSecrets: { 'my-okta': true },
    },
    restartRequired: false,
    ...overrides,
  }
}

async function mountView() {
  const wrapper = mount(SetupView)
  await flushPromises()
  return wrapper
}

describe('SetupView', () => {
  beforeEach(() => {
    useToasts().toasts.value = []
  })
  afterEach(() => {
    useToasts().toasts.value = []
  })

  it('shows a spinner while loading', () => {
    fetchSettings.mockReturnValue(new Promise(() => {}))
    expect(mount(SetupView).find('[data-test="setup-loading"]').exists()).toBe(true)
  })

  it('asks for the root token without a session, then loads settings', async () => {
    fetchSettings.mockResolvedValueOnce(null).mockResolvedValueOnce(state())
    setupLogin.mockResolvedValue()
    const wrapper = await mountView()
    expect(wrapper.find('[data-test="setup-login"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="setup-unlock"]').attributes('disabled')).toBeDefined()

    await wrapper.find('[data-test="setup-token"]').setValue('root-token')
    await wrapper.find('[data-test="setup-login"]').trigger('submit')
    await flushPromises()
    expect(setupLogin).toHaveBeenCalledWith('root-token')
    expect(wrapper.find('[data-test="setup-form"]').exists()).toBe(true)
    expect(wrapper.find('#app-url').element.value).toBe('https://tracker.topup.sh/')
  })

  it('shows a wrong-token error', async () => {
    fetchSettings.mockResolvedValue(null)
    setupLogin.mockRejectedValue(new Error('invalid root access token'))
    const wrapper = await mountView()
    await wrapper.find('[data-test="setup-token"]').setValue('nope')
    await wrapper.find('[data-test="setup-login"]').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[data-test="setup-login-error"]').text()).toBe('invalid root access token')
  })

  it('falls back to sign-in when loading fails', async () => {
    fetchSettings.mockRejectedValue(new Error('Failed to load settings (500)'))
    const wrapper = await mountView()
    expect(wrapper.find('[data-test="setup-login"]').exists()).toBe(true)
    expect(useToasts().toasts.value[0].message).toBe('Failed to load settings (500)')
  })

  it('renders sections, masks saved secrets and switches database driver', async () => {
    fetchSettings.mockResolvedValue(state())
    const wrapper = await mountView()
    expect(wrapper.find('[data-test="setup-sqlite"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="setup-restart"]').exists()).toBe(false)

    await wrapper.find('[data-test="setup-driver-postgres"]').trigger('click')
    expect(wrapper.find('[data-test="setup-postgres"]').exists()).toBe(true)
    expect(wrapper.find('#pg-dsn').attributes('placeholder')).toContain('saved')
    expect(wrapper.find('#pg-dsn').attributes('type')).toBe('password')
    expect(wrapper.find('#google-secret').attributes('placeholder')).toContain('saved')
    expect(wrapper.find('#github-secret').attributes('placeholder')).toBe('Not set')

    await wrapper.find('[data-test="setup-driver-sqlite"]').trigger('click')
    expect(wrapper.find('[data-test="setup-sqlite"]').exists()).toBe(true)
  })

  it('shows an example DSN when none is saved', async () => {
    fetchSettings.mockResolvedValue(state({ secretsSet: { ...state().secretsSet, postgresDsn: false } }))
    const wrapper = await mountView()
    await wrapper.find('[data-test="setup-driver-postgres"]').trigger('click')
    expect(wrapper.find('#pg-dsn').attributes('placeholder')).toMatch(/^postgres:\/\//)
  })

  it('shows one settings section at a time from the side menu', async () => {
    fetchSettings.mockResolvedValue(state())
    const wrapper = await mountView()
    const ids = ['domain', 'tls', 'database', 'signin']
    const visible = () => ids.filter((id) => wrapper.find(`[data-test="setup-${id}"]`).element.style.display !== 'none')
    expect(wrapper.findAll('nav[aria-label="Settings"] button').map((b) => b.text())).toEqual([
      'Domains',
      'TLS',
      'Database',
      'Sign-in',
    ])
    expect(visible()).toEqual(['domain'])
    expect(wrapper.find('[data-test="setup-tab-domain"]').attributes('aria-current')).toBe('page')

    await wrapper.find('[data-test="setup-tab-database"]').trigger('click')
    expect(visible()).toEqual(['database'])
    expect(wrapper.find('[data-test="setup-tab-database"]').attributes('aria-current')).toBe('page')
    expect(wrapper.find('[data-test="setup-tab-domain"]').attributes('aria-current')).toBeUndefined()
  })

  it('saves the edited settings and shows restart notice', async () => {
    fetchSettings.mockResolvedValue(state())
    saveSettings.mockImplementation(async (settings) => state({ settings: { ...settings, auth: { ...settings.auth, github: { clientId: 'h', clientSecret: '' } } }, restartRequired: true }))
    const wrapper = await mountView()

    await wrapper.find('#github-id').setValue('h')
    await wrapper.find('#github-secret').setValue('hsecret')
    await wrapper.find('[data-test="setup-form"]').trigger('submit')
    await flushPromises()

    const sent = saveSettings.mock.calls[0][0]
    expect(sent.auth.github).toEqual({ clientId: 'h', clientSecret: 'hsecret' })
    expect(wrapper.find('[data-test="setup-restart"]').exists()).toBe(true)
    expect(wrapper.find('#github-secret').element.value).toBe('', 'secret cleared after save')
    expect(useToasts().toasts.value.at(-1)).toMatchObject({ type: 'success', message: 'Settings saved' })
  })

  it('binds every field', async () => {
    fetchSettings.mockResolvedValue(state())
    saveSettings.mockImplementation(async (settings) => state({ settings }))
    const wrapper = await mountView()
    await wrapper.find('#sqlite-dsn').setValue('/data/x.db')
    await wrapper.find('[data-test="setup-driver-postgres"]').trigger('click')
    const values = {
      '#pg-dsn': 'postgres://app:pw@db:6543/prod?sslmode=require',
      '#google-id': 'g2', '#google-secret': 'gs2',
    }
    for (const [sel, value] of Object.entries(values)) await wrapper.find(sel).setValue(value)
    await wrapper.find('[data-test="setup-form"]').trigger('submit')
    await flushPromises()

    expect(saveSettings.mock.calls[0][0]).toMatchObject({
      database: {
        driver: 'postgres',
        sqlite: { dsn: '/data/x.db' },
        postgres: { dsn: 'postgres://app:pw@db:6543/prod?sslmode=require' },
      },
      auth: { google: { clientId: 'g2', clientSecret: 'gs2' } },
    })
  })

  it('shows validation errors from the server', async () => {
    fetchSettings.mockResolvedValue(state())
    saveSettings.mockRejectedValue(Object.assign(new Error('invalid settings: sqlite DSN is required'), { status: 400 }))
    const wrapper = await mountView()
    await wrapper.find('[data-test="setup-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[data-test="setup-save-error"]').text()).toContain('sqlite DSN is required')
    expect(useToasts().toasts.value.at(-1).type).toBe('error')
  })

  it('returns to sign-in when the root session expired', async () => {
    fetchSettings.mockResolvedValue(state())
    saveSettings.mockRejectedValue(Object.assign(new Error('unauthorized'), { status: 401 }))
    const wrapper = await mountView()
    await wrapper.find('[data-test="setup-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.find('[data-test="setup-login-error"]').text()).toContain('session expired')
  })

  it('edits domains and TLS', async () => {
    fetchSettings.mockResolvedValue(state())
    saveSettings.mockImplementation(async (settings) => state({ settings }))
    const wrapper = await mountView()
    const tls = wrapper.find('[data-test="setup-tls"]')
    expect(tls.find('.opacity-50').exists()).toBe(true)
    expect(wrapper.find('#tls-cert').exists()).toBe(false)

    expect(wrapper.find('#allowed-origins').element.value).toBe('https://topup.sh')
    await wrapper.find('#app-url').setValue('https://example.org')
    await wrapper.find('#allowed-origins').setValue(' https://example.org \n\nhttps://www.example.org\n')
    await wrapper.find('[data-test="setup-tls-enabled"]').setValue(true)
    expect(tls.find('.opacity-50').exists()).toBe(false)
    const values = {
      '#tls-port': '8443', '#tls-email': 'ops@example.org',
    }
    for (const [sel, value] of Object.entries(values)) await wrapper.find(sel).setValue(value)
    await wrapper.find('[data-test="setup-form"]').trigger('submit')
    await flushPromises()

    expect(saveSettings.mock.calls[0][0]).toMatchObject({
      domain: { appUrl: 'https://example.org', allowedOrigins: ['https://example.org', 'https://www.example.org'] },
      tls: { enabled: true, port: 8443, email: 'ops@example.org' },
    })
  })

  it('saves no allowed origins when none are set', async () => {
    const empty = state()
    empty.settings.domain = { appUrl: 'https://tracker.topup.sh', allowedOrigins: null }
    fetchSettings.mockResolvedValue(empty)
    saveSettings.mockImplementation(async (settings) => state({ settings }))
    const wrapper = await mountView()
    expect(wrapper.find('#allowed-origins').element.value).toBe('')
    expect(wrapper.find('[data-test="setup-domain"]').text()).toContain('Applies right away')
    await wrapper.find('[data-test="setup-form"]').trigger('submit')
    await flushPromises()
    expect(saveSettings.mock.calls[0][0].domain.allowedOrigins).toEqual([])
  })

  it('adds, edits, uploads icons for and removes custom providers', async () => {
    fetchSettings.mockResolvedValue(state())
    saveSettings.mockImplementation(async (settings) => state({ settings }))
    const wrapper = await mountView()

    expect(wrapper.find('#cp-0-secret').attributes('placeholder')).toContain('saved')
    expect(wrapper.find('[data-test="cp-0-callback"]').text()).toContain('https://tracker.topup.sh/auth/my-okta/callback')
    expect(wrapper.find('[data-test="cp-0-preview"]').attributes('src')).toBe(`data:image/svg+xml;charset=utf-8,${encodeURIComponent('<svg></svg>')}`)

    await wrapper.find('[data-test="setup-add-provider"]').trigger('click')
    expect(wrapper.find('[data-test="cp-1-preview"]').exists()).toBe(false)
    expect(wrapper.find('#cp-1-secret').attributes('placeholder')).toBe('Not set')
    expect(wrapper.find('[data-test="cp-1-callback"]').text()).toContain('/auth//callback')
    const values = {
      '#cp-1-name': ' Acme  SSO! ', '#cp-1-auth-url': 'https://sso.acme.dev', '#cp-1-id': 'aid', '#cp-1-secret': 'asecret',
    }
    for (const [sel, value] of Object.entries(values)) await wrapper.find(sel).setValue(value)
    expect(wrapper.find('[data-test="cp-1-callback"]').text()).toContain('/auth/acme-sso/callback')

    // Upload reads the file into the icon; an empty pick changes nothing.
    const upload = wrapper.find('[data-test="cp-1-upload"]')
    const svg = '<svg viewBox="0 0 1 1"></svg>'
    Object.defineProperty(upload.element, 'files', { value: [{ text: async () => svg }], configurable: true })
    await upload.trigger('change')
    await flushPromises()
    expect(wrapper.find('#cp-1-icon').element.value).toBe(svg)
    Object.defineProperty(upload.element, 'files', { value: [], configurable: true })
    await upload.trigger('change')
    await flushPromises()
    expect(wrapper.find('#cp-1-icon').element.value).toBe(svg)
    await wrapper.find('#cp-1-icon').setValue('<svg>edited</svg>')

    await wrapper.find('[data-test="cp-0-remove"]').trigger('click')
    expect(wrapper.find('[data-test="setup-custom-provider-1"]').exists()).toBe(false)
    await wrapper.find('[data-test="setup-form"]').trigger('submit')
    await flushPromises()

    expect(saveSettings.mock.calls[0][0].auth.providers).toEqual([
      { name: ' Acme  SSO! ', icon: '<svg>edited</svg>', clientId: 'aid', clientSecret: 'asecret', authUrl: 'https://sso.acme.dev' },
    ])
  })

  it('treats missing providers and provider secrets as empty', async () => {
    const s = state()
    s.settings.auth.providers = null
    delete s.secretsSet.providerClientSecrets
    fetchSettings.mockResolvedValue(s)
    const wrapper = await mountView()
    expect(wrapper.find('[data-test="setup-custom-provider-0"]').exists()).toBe(false)
    await wrapper.find('[data-test="setup-add-provider"]').trigger('click')
    expect(wrapper.find('#cp-0-secret').attributes('placeholder')).toBe('Not set')
  })

  it('signs out, even if the request fails', async () => {
    for (const fail of [false, true]) {
      fetchSettings.mockResolvedValue(state())
      if (fail) setupLogout.mockRejectedValueOnce(new Error('offline'))
      else setupLogout.mockResolvedValueOnce()
      const wrapper = await mountView()
      await wrapper.find('[data-test="setup-logout"]').trigger('click')
      await flushPromises()
      expect(wrapper.find('[data-test="setup-login"]').exists()).toBe(true)
      wrapper.unmount()
    }
  })
})
