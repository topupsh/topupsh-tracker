<!--
  Copyright 2026 Contextual, Inc.
  This notice may not be modified or removed.
  SPDX-License-Identifier: AGPL-3.0-only
-->

<script setup>
// A service's orgs: each one's org limit, user limit and this month's
// usage, with a form to set any org's limits.
import { onMounted, reactive, ref } from 'vue'
import { listOrgUsage, setOrgLimits } from '../api'
import { useToasts } from '../composables/useToasts'
import { CARD, HEADING, INPUT, LABEL, PRIMARY, formatCredits, formatLimit, limitText, parseLimit } from '../utils/credits'

const props = defineProps({ service: { type: String, required: true } })
const emit = defineEmits(['error'])

const { notifyError, notifySuccess } = useToasts()

// Columns: org, org limit, user limit, used; an edit cue from md up.
const ROW = 'grid grid-cols-[minmax(0,1fr)_4.5rem_4.5rem_4.5rem] md:grid-cols-[minmax(0,1fr)_7rem_7rem_9rem_3rem] items-center gap-3 px-4'

const orgs = ref([])
const loading = ref(true)
const saving = ref(false)
// The limits form, filled from a row by edit().
const form = reactive({ org: '', orgLimit: '', userLimit: '' })

async function load() {
  try {
    orgs.value = await listOrgUsage(props.service)
  } catch (err) {
    emit('error', err)
  } finally {
    loading.value = false
  }
}

function edit(o) {
  form.org = o.org
  form.orgLimit = String(o.orgLimit)
  form.userLimit = limitText(o.userLimit)
}

async function save() {
  const org = form.org.trim()
  const orgLimit = parseLimit(form.orgLimit)
  const userLimit = parseLimit(form.userLimit)
  if (orgLimit === null || orgLimit === undefined || userLimit === undefined) {
    notifyError('Limits are whole numbers of 0 or more; the org limit is required')
    return
  }
  saving.value = true
  try {
    await setOrgLimits(props.service, org, { orgLimit, userLimit })
    notifySuccess(`Saved the limits of ${org}`)
    Object.assign(form, { org: '', orgLimit: '', userLimit: '' })
    await load()
  } catch (err) {
    emit('error', err)
  } finally {
    saving.value = false
  }
}

// usedPercent is how full the org's monthly limit is, for the bar.
function usedPercent(o) {
  return o.orgLimit > 0 ? Math.min(100, Math.round((o.used / o.orgLimit) * 100)) : 100
}

onMounted(load)
</script>

<template>
  <div class="flex flex-col gap-4" data-test="credit-orgs">
    <form :class="[CARD, 'p-4 flex flex-col gap-3']" data-test="org-limits-form" @submit.prevent="save">
      <div>
        <h2 :class="HEADING">Set org limits</h2>
        <p class="mt-1 text-[11px] text-gray-500 dark:text-zinc-400">
          The org limit caps the whole org each month; the user limit caps each of its users. Leave the user limit blank for no cap. Pick an org below to edit its limits.
        </p>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_9rem_9rem_auto] sm:items-end gap-3">
        <div>
          <label for="org-limits-org" :class="LABEL">Org</label>
          <input id="org-limits-org" v-model="form.org" required maxlength="64" placeholder="The service's org ID" :class="INPUT" data-test="org-limits-org" />
        </div>
        <div>
          <label for="org-limits-org-limit" :class="LABEL">Org limit / month</label>
          <input id="org-limits-org-limit" v-model="form.orgLimit" required inputmode="numeric" :class="INPUT" data-test="org-limits-org-limit" />
        </div>
        <div>
          <label for="org-limits-user-limit" :class="LABEL">User limit / month</label>
          <input id="org-limits-user-limit" v-model="form.userLimit" inputmode="numeric" placeholder="Uncapped" :class="INPUT" data-test="org-limits-user-limit" />
        </div>
        <button type="submit" :class="PRIMARY" :disabled="saving || !form.org.trim()" data-test="org-limits-save">Save</button>
      </div>
    </form>

    <section :class="[CARD, 'overflow-hidden']">
      <div :class="[ROW, HEADING, 'py-2.5 border-b border-gray-100 dark:border-zinc-800']">
        <span>Org</span>
        <span class="text-right">Org limit</span>
        <span class="text-right">User limit</span>
        <span class="text-right">Used</span>
        <span class="hidden md:block" />
      </div>
      <p v-if="loading" class="px-4 py-6 text-[10px] font-black text-gray-300 dark:text-zinc-500 animate-pulse" data-test="orgs-loading">Loading...</p>
      <p v-else-if="orgs.length === 0" class="px-4 py-6 text-xs text-gray-400 dark:text-zinc-500" data-test="orgs-empty">
        No org has limits or usage this month. Orgs use the service's default limits until you set theirs.
      </p>
      <ul v-else class="divide-y divide-gray-100 dark:divide-zinc-800">
        <li v-for="o in orgs" :key="o.org">
          <button
            type="button"
            :class="[ROW, 'group w-full py-2.5 text-left text-xs hover:bg-zinc-50 dark:hover:bg-zinc-800/50 transition-colors']"
            :data-test="`org-${o.org}`"
            @click="edit(o)"
          >
            <span class="min-w-0 flex flex-col items-start gap-0.5 md:flex-row md:items-center md:gap-2">
              <span class="max-w-full truncate font-mono font-semibold text-gray-900 dark:text-zinc-100">{{ o.org }}</span>
              <span v-if="!o.custom" class="shrink-0 rounded bg-gray-100 dark:bg-zinc-800 px-1.5 py-0.5 text-[9px] font-black uppercase tracking-widest text-gray-500 dark:text-zinc-400">Default</span>
            </span>
            <span class="text-right text-gray-900 dark:text-zinc-100 tabular-nums">{{ formatCredits(o.orgLimit) }}</span>
            <span class="text-right text-gray-500 dark:text-zinc-400 tabular-nums">{{ formatLimit(o.userLimit) }}</span>
            <span class="text-right">
              <span class="block tabular-nums text-gray-900 dark:text-zinc-100">{{ formatCredits(o.used) }}</span>
              <span class="hidden md:block mt-1 h-1 rounded-full bg-gray-100 dark:bg-zinc-800 overflow-hidden">
                <span class="block h-full rounded-full" :class="usedPercent(o) >= 100 ? 'bg-rose-500' : 'bg-gray-900 dark:bg-zinc-100'" :style="{ width: `${usedPercent(o)}%` }" />
              </span>
            </span>
            <span class="hidden md:block text-right text-[11px] font-bold text-gray-400 dark:text-zinc-500 group-hover:text-gray-900 dark:group-hover:text-zinc-100">Edit</span>
          </button>
        </li>
      </ul>
    </section>
  </div>
</template>
