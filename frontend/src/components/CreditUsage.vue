<!--
  Copyright 2026 Contextual, Inc.
  This notice may not be modified or removed.
  SPDX-License-Identifier: AGPL-3.0-only
-->

<script setup>
// A service's users with credits: what each spent of this month's
// allowance, their grants and balance, and a form to grant credits.
import { onMounted, reactive, ref } from 'vue'
import { createGrant, listUserUsage } from '../api'
import { useToasts } from '../composables/useToasts'
import { CARD, HEADING, INPUT, LABEL, PRIMARY, SECONDARY, formatCredits, formatDateTime } from '../utils/credits'

const props = defineProps({ service: { type: String, required: true } })
const emit = defineEmits(['error'])

const { notifyError, notifySuccess } = useToasts()

// Columns: user, monthly used of allowance, grants, balance.
const ROW = 'grid grid-cols-[minmax(0,1fr)_5.5rem_4rem] md:grid-cols-[minmax(0,1fr)_10rem_6rem_6rem] items-center gap-3 px-4'

const users = ref([])
const truncated = ref(false)
const loading = ref(true)
const org = ref('')
// The org the shown list answers.
const filtered = ref('')
const granting = ref(false)
const grant = reactive({ org: '', user: '', credits: '' })

async function load() {
  loading.value = true
  filtered.value = org.value.trim()
  try {
    const page = await listUserUsage(props.service, filtered.value)
    users.value = page.users
    truncated.value = page.truncated
  } catch (err) {
    emit('error', err)
  } finally {
    loading.value = false
  }
}

function pick(u) {
  grant.org = u.org
  grant.user = u.user
}

async function submitGrant() {
  const credits = Number(grant.credits)
  if (!Number.isInteger(credits) || credits < 1) {
    notifyError('Grant a whole number of credits, 1 or more')
    return
  }
  granting.value = true
  try {
    const g = await createGrant({ service: props.service, org: grant.org.trim(), user: grant.user.trim(), credits })
    notifySuccess(`Granted ${formatCredits(g.credits)} credits to ${g.user}, until ${formatDateTime(g.expiresAt)}`)
    grant.credits = ''
    await load()
  } catch (err) {
    emit('error', err)
  } finally {
    granting.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="flex flex-col gap-4" data-test="credit-usage">
    <form :class="[CARD, 'p-4 flex flex-col gap-3']" data-test="grant-form" @submit.prevent="submitGrant">
      <div>
        <h2 :class="HEADING">Grant credits</h2>
        <p class="mt-1 text-[11px] text-gray-500 dark:text-zinc-400">
          Extra credits for one user, on top of the monthly limits. They expire in a month. Pick a user below to fill in their org.
        </p>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_7rem_auto] sm:items-end gap-3">
        <div>
          <label for="grant-org" :class="LABEL">Org</label>
          <input id="grant-org" v-model="grant.org" required maxlength="64" :class="INPUT" data-test="grant-org" />
        </div>
        <div>
          <label for="grant-user" :class="LABEL">User</label>
          <input id="grant-user" v-model="grant.user" required maxlength="64" :class="INPUT" data-test="grant-user" />
        </div>
        <div>
          <label for="grant-credits" :class="LABEL">Credits</label>
          <input id="grant-credits" v-model="grant.credits" required inputmode="numeric" :class="INPUT" data-test="grant-credits" />
        </div>
        <button type="submit" :class="PRIMARY" :disabled="granting || !grant.org.trim() || !grant.user.trim()" data-test="grant-submit">Grant</button>
      </div>
    </form>

    <form class="flex gap-2" role="search" data-test="usage-filter" @submit.prevent="load">
      <label for="usage-org" class="sr-only">Filter by org</label>
      <input id="usage-org" v-model="org" type="search" placeholder="Org ID" :class="INPUT" data-test="usage-org" />
      <button type="submit" :class="SECONDARY" :disabled="loading" data-test="usage-filter-submit">Filter</button>
    </form>

    <section :class="[CARD, 'overflow-hidden']">
      <div :class="[ROW, HEADING, 'py-2.5 border-b border-gray-100 dark:border-zinc-800']">
        <span>User</span>
        <span class="text-right">Monthly used</span>
        <span class="hidden md:block text-right">Grants</span>
        <span class="text-right">Balance</span>
      </div>
      <p v-if="loading" class="px-4 py-6 text-[10px] font-black text-gray-300 dark:text-zinc-500 animate-pulse" data-test="usage-loading">Loading...</p>
      <p v-else-if="users.length === 0" class="px-4 py-6 text-xs text-gray-400 dark:text-zinc-500" data-test="usage-empty">
        {{ filtered ? `No user of “${filtered}” has credits.` : 'No user has credits yet: a user appears after their first charge or grant.' }}
      </p>
      <ul v-else class="divide-y divide-gray-100 dark:divide-zinc-800">
        <li v-for="u in users" :key="`${u.org}/${u.user}`">
          <button
            type="button"
            :class="[ROW, 'w-full py-2.5 text-left text-xs hover:bg-zinc-50 dark:hover:bg-zinc-800/50 transition-colors']"
            :data-test="`usage-${u.org}-${u.user}`"
            @click="pick(u)"
          >
            <span class="min-w-0">
              <span class="block truncate font-mono font-semibold text-gray-900 dark:text-zinc-100">{{ u.user }}</span>
              <span class="block truncate text-[10px] text-gray-400 dark:text-zinc-500 font-mono">{{ u.org }}</span>
            </span>
            <span class="text-right tabular-nums">
              <span :class="u.monthlyUsed >= u.monthlyAllowance ? 'text-rose-600 dark:text-rose-400' : 'text-gray-900 dark:text-zinc-100'">{{ formatCredits(u.monthlyUsed) }}</span>
              <span class="text-gray-400 dark:text-zinc-500"> / {{ formatCredits(u.monthlyAllowance) }}</span>
            </span>
            <span class="hidden md:block text-right tabular-nums text-gray-500 dark:text-zinc-400">{{ formatCredits(u.grants) }}</span>
            <span class="text-right tabular-nums font-semibold text-gray-900 dark:text-zinc-100">{{ formatCredits(u.balance) }}</span>
          </button>
        </li>
      </ul>
      <p v-if="truncated && !loading" class="px-4 py-3 border-t border-gray-100 dark:border-zinc-800 text-[11px] text-gray-500 dark:text-zinc-400" data-test="usage-truncated">
        Showing the first {{ users.length }} users. Filter by org to see the others.
      </p>
    </section>
  </div>
</template>
