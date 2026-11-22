<!--
  Copyright 2026 Contextual, Inc.
  This notice may not be modified or removed.
  SPDX-License-Identifier: AGPL-3.0-only
-->

<script setup>
// The credit services: each one's switch, default limits and API keys. A
// new key is shown once; creating one for a new name creates the service.
import { onMounted, reactive, ref } from 'vue'
import DeleteModal from './DeleteModal.vue'
import { createServiceKey, listServices, revokeServiceKey, setServiceDefaults, setServiceEnabled } from '../api'
import { useClipboard } from '../composables/useClipboard'
import { useToasts } from '../composables/useToasts'
import { CARD, HEADING, INPUT, LABEL, PRIMARY, SECONDARY, formatDateTime, limitText, parseLimit } from '../utils/credits'

const emit = defineEmits(['error'])

const { copy } = useClipboard()
const { notifyError, notifySuccess } = useToasts()

const services = ref([])
const loading = ref(true)
// The default limit inputs, by service name.
const defaults = reactive({})
// What is in flight: 'switch:<name>', 'defaults:<name>', 'key:<name>'.
const busy = ref('')
const newName = ref('')
// The key just created: { service, key }, shown until dismissed.
const created = ref(null)
// The key the revoke modal asks about: { service, key }.
const revoking = ref(null)

async function load() {
  try {
    services.value = await listServices()
    for (const s of services.value) {
      defaults[s.name] = { orgLimit: String(s.defaultOrgLimit), userLimit: limitText(s.defaultUserLimit) }
    }
  } catch (err) {
    emit('error', err)
  } finally {
    loading.value = false
  }
}

// run marks what is busy while fn runs and reports a failure.
async function run(what, fn) {
  busy.value = what
  try {
    await fn()
  } catch (err) {
    emit('error', err)
  } finally {
    busy.value = ''
  }
}

function toggle(s) {
  return run(`switch:${s.name}`, async () => {
    const saved = await setServiceEnabled(s.name, !s.enabled)
    s.enabled = saved.enabled
    notifySuccess(`Credit limiting is ${saved.enabled ? 'on' : 'off'} for ${s.name}`)
  })
}

function saveDefaults(s) {
  const orgLimit = parseLimit(defaults[s.name].orgLimit)
  const userLimit = parseLimit(defaults[s.name].userLimit)
  if (orgLimit === null || orgLimit === undefined || userLimit === undefined) {
    notifyError('Limits are whole numbers of 0 or more; the org limit is required')
    return
  }
  return run(`defaults:${s.name}`, async () => {
    const saved = await setServiceDefaults(s.name, { orgLimit, userLimit })
    s.defaultOrgLimit = saved.defaultOrgLimit
    s.defaultUserLimit = saved.defaultUserLimit
    notifySuccess(`Saved the default limits of ${s.name}`)
  })
}

function createKey(name) {
  return run(`key:${name}`, async () => {
    const rs = await createServiceKey(name)
    created.value = { service: name, key: rs.key }
    newName.value = ''
    await load()
  })
}

function confirmRevoke() {
  const { service, key } = revoking.value
  return run(`revoke:${key.id}`, async () => {
    await revokeServiceKey(service, key.id)
    revoking.value = null
    notifySuccess(`Revoked ${key.prefix}…`)
    await load()
  })
}

onMounted(load)
</script>

<template>
  <div class="flex flex-col gap-4" data-test="credit-services">
    <form :class="[CARD, 'p-4 flex flex-col sm:flex-row sm:items-end gap-3']" data-test="service-new" @submit.prevent="createKey(newName.trim())">
      <div class="grow">
        <label for="service-new-name" :class="LABEL">New service</label>
        <input id="service-new-name" v-model="newName" placeholder="e.g. mesher" pattern="[a-z0-9][a-z0-9\-]{0,63}" required :class="INPUT" data-test="service-new-name" />
      </div>
      <button type="submit" :class="PRIMARY" :disabled="!newName.trim() || busy !== ''" data-test="service-new-submit">Create with key</button>
    </form>

    <section
      v-if="created"
      class="rounded-xl border border-amber-200 dark:border-amber-500/30 bg-amber-50 dark:bg-amber-500/10 p-4"
      role="status"
      data-test="key-created"
    >
      <p class="text-xs font-semibold text-amber-900 dark:text-amber-200">
        New API key for {{ created.service }}. Copy it now: it won't be shown again.
      </p>
      <div class="mt-2 flex flex-col sm:flex-row gap-2">
        <code class="grow min-w-0 break-all rounded-lg bg-white dark:bg-zinc-900 border border-amber-200 dark:border-amber-500/30 px-3 py-2 text-xs font-mono text-gray-900 dark:text-zinc-100" data-test="key-created-value">{{ created.key }}</code>
        <div class="flex gap-2 shrink-0">
          <button type="button" :class="SECONDARY" data-test="key-created-copy" @click="copy(created.key, 'API key copied')">Copy</button>
          <button type="button" :class="SECONDARY" data-test="key-created-done" @click="created = null">Done</button>
        </div>
      </div>
    </section>

    <p v-if="loading" class="px-1 py-6 text-[10px] font-black text-gray-300 dark:text-zinc-500 animate-pulse" data-test="services-loading">Loading...</p>
    <p v-else-if="services.length === 0" :class="[CARD, 'px-4 py-6 text-xs text-gray-400 dark:text-zinc-500']" data-test="services-empty">
      No services yet. Create one above: its first API key comes with it.
    </p>

    <section v-for="s in services" :key="s.name" :class="CARD" :data-test="`service-${s.name}`">
      <header class="flex items-center gap-3 px-4 py-3 border-b border-gray-100 dark:border-zinc-800">
        <div class="grow min-w-0">
          <h2 class="text-sm font-bold text-gray-900 dark:text-zinc-100 truncate">{{ s.name }}</h2>
          <p class="text-[10px] text-gray-400 dark:text-zinc-500">Since {{ formatDateTime(s.createdAt) }}</p>
        </div>
        <span class="text-[11px] font-semibold" :class="s.enabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-400 dark:text-zinc-500'">
          {{ s.enabled ? 'Limiting on' : 'Limiting off' }}
        </span>
        <button
          type="button"
          role="switch"
          :aria-checked="s.enabled"
          :aria-label="`Credit limiting for ${s.name}`"
          :disabled="busy === `switch:${s.name}`"
          :class="[
            'relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors disabled:opacity-50',
            s.enabled ? 'bg-emerald-500' : 'bg-gray-200 dark:bg-zinc-700',
          ]"
          :data-test="`service-switch-${s.name}`"
          @click="toggle(s)"
        >
          <span :class="['inline-block h-4 w-4 rounded-full bg-white transition-transform', s.enabled ? 'translate-x-4.5' : 'translate-x-0.5']" />
        </button>
      </header>

      <div class="grid md:grid-cols-2 divide-y md:divide-y-0 md:divide-x divide-gray-100 dark:divide-zinc-800">
        <form class="p-4 flex flex-col gap-3" :data-test="`service-defaults-${s.name}`" @submit.prevent="saveDefaults(s)">
          <div>
            <h3 :class="HEADING">Default limits</h3>
            <p class="mt-1 text-[11px] text-gray-500 dark:text-zinc-400">For orgs without limits of their own. Leave the user limit blank for no cap.</p>
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label :for="`defaults-org-${s.name}`" :class="LABEL">Org limit / month</label>
              <input :id="`defaults-org-${s.name}`" v-model="defaults[s.name].orgLimit" inputmode="numeric" :class="INPUT" :data-test="`defaults-org-${s.name}`" />
            </div>
            <div>
              <label :for="`defaults-user-${s.name}`" :class="LABEL">User limit / month</label>
              <input :id="`defaults-user-${s.name}`" v-model="defaults[s.name].userLimit" inputmode="numeric" placeholder="Uncapped" :class="INPUT" :data-test="`defaults-user-${s.name}`" />
            </div>
          </div>
          <div>
            <button type="submit" :class="SECONDARY" :disabled="busy === `defaults:${s.name}`" :data-test="`defaults-save-${s.name}`">Save defaults</button>
          </div>
        </form>

        <div class="p-4 flex flex-col gap-3">
          <div class="flex items-start gap-3">
            <div class="grow">
              <h3 :class="HEADING">API keys</h3>
              <p class="mt-1 text-[11px] text-gray-500 dark:text-zinc-400">To rotate, create a key, switch callers to it, then revoke the old one.</p>
            </div>
            <button type="button" :class="[SECONDARY, 'shrink-0 whitespace-nowrap']" :disabled="busy === `key:${s.name}`" :data-test="`key-create-${s.name}`" @click="createKey(s.name)">New key</button>
          </div>
          <p v-if="s.apiKeys.length === 0" class="text-xs text-gray-400 dark:text-zinc-500">No keys.</p>
          <ul v-else class="divide-y divide-gray-100 dark:divide-zinc-800 -mx-1">
            <li v-for="k in s.apiKeys" :key="k.id" class="flex items-center gap-3 px-1 py-2 text-xs" :data-test="`key-${k.id}`">
              <span class="grow min-w-0">
                <span class="block font-mono truncate" :class="k.revokedAt ? 'text-gray-400 dark:text-zinc-500 line-through' : 'text-gray-900 dark:text-zinc-100'">{{ k.prefix }}…</span>
                <span class="block text-[10px] text-gray-400 dark:text-zinc-500">
                  Created {{ formatDateTime(k.createdAt) }}<template v-if="k.revokedAt"> · revoked {{ formatDateTime(k.revokedAt) }}</template>
                </span>
              </span>
              <button
                v-if="!k.revokedAt"
                type="button"
                class="text-[11px] font-bold text-rose-600 dark:text-rose-400 hover:underline"
                :data-test="`key-revoke-${k.id}`"
                @click="revoking = { service: s.name, key: k }"
              >
                Revoke
              </button>
            </li>
          </ul>
        </div>
      </div>
    </section>

    <DeleteModal
      :open="revoking !== null"
      title="Revoke API key?"
      :message="revoking ? `${revoking.key.prefix}… stops working for ${revoking.service} at once.` : ''"
      confirm-label="Revoke"
      :busy="busy.startsWith('revoke:')"
      @confirm="confirmRevoke"
      @cancel="revoking = null"
    />
  </div>
</template>
