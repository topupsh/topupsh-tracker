<!--
  Copyright 2026 Contextual, Inc.
  This notice may not be modified or removed.
  SPDX-License-Identifier: AGPL-3.0-only
-->

<script setup>
// A service's user sees their own credits here, read-only. They have no
// topup.sh account: the service links them to /balance#token=<view token>.
// The token moves to sessionStorage and out of the address bar, and reads
// stop working when it expires (an hour after it was issued).
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BrandLogo from '../components/BrandLogo.vue'
import StatTile from '../components/StatTile.vue'
import { fetchViewerBalance, listViewerLedger } from '../api'
import { CARD, HEADING, SECONDARY, formatCredits, formatDateTime } from '../utils/credits'

const TOKEN_KEY = 'topupsh.viewToken'
const KIND_LABEL = {
  spend: 'Spent',
  refund: 'Refunded',
  grant: 'Granted',
  renewal: 'Monthly credits',
  limit: 'Limit changed',
  expiry: 'Expired',
}

// Spends and refunds name the operation; other entries only carry the
// operation of the charge that happened to trigger them.
const SHOWS_OPERATION = new Set(['spend', 'refund'])
// History columns: what, change, balance after.
const HISTORY_ROW = 'grid grid-cols-[minmax(0,1fr)_4.5rem_4.5rem] items-center gap-3 px-4'

const route = useRoute()
const router = useRouter()

// 'loading' | 'ready' | 'missing' (no token) | 'expired' | 'error'
const phase = ref('loading')
const error = ref('')
const balance = ref(null)
const entries = ref([])
const nextCursor = ref('')
const loadingMore = ref(false)
let token = ''

function readStored() {
  try {
    return sessionStorage.getItem(TOKEN_KEY) || ''
  } catch {
    return ''
  }
}

function store(value) {
  try {
    if (value) sessionStorage.setItem(TOKEN_KEY, value)
    else sessionStorage.removeItem(TOKEN_KEY)
  } catch {
    // Without storage the token lives as long as the page.
  }
}

// takeToken prefers a token in the link over a stored one, and strips it
// from the address bar so it isn't bookmarked or shared by accident.
function takeToken() {
  const fromLink = new URLSearchParams(route.hash.replace(/^#/, '')).get('token') || ''
  if (fromLink) {
    store(fromLink)
    router.replace({ path: route.path, query: route.query })
    return fromLink
  }
  return readStored()
}

function fail(err) {
  if (err.status === 401) {
    store('')
    phase.value = 'expired'
    return
  }
  error.value = err.message
  phase.value = 'error'
}

async function load() {
  token = takeToken()
  if (!token) {
    phase.value = 'missing'
    return
  }
  try {
    const [b, page] = await Promise.all([fetchViewerBalance(token), listViewerLedger(token)])
    balance.value = b
    entries.value = page.entries
    nextCursor.value = page.nextCursor || ''
    phase.value = 'ready'
  } catch (err) {
    fail(err)
  }
}

async function loadMore() {
  loadingMore.value = true
  try {
    const page = await listViewerLedger(token, nextCursor.value)
    entries.value.push(...page.entries)
    nextCursor.value = page.nextCursor || ''
  } catch (err) {
    fail(err)
  } finally {
    loadingMore.value = false
  }
}

const monthlyPercent = computed(() => {
  const m = balance.value.monthly
  return m.limit > 0 ? Math.min(100, Math.round((m.used / m.limit) * 100)) : 100
})

function formatDate(iso) {
  return new Date(iso).toLocaleDateString(undefined, { dateStyle: 'medium' })
}

function signed(n) {
  return n > 0 ? `+${formatCredits(n)}` : formatCredits(n)
}

onMounted(load)
</script>

<template>
  <div class="min-h-[100dvh] bg-zinc-50 dark:bg-zinc-950 px-4 py-10">
    <div class="mx-auto w-full max-w-3xl">
      <header class="flex items-center gap-3 mb-6">
        <BrandLogo tile class="w-9 h-9" />
        <div class="min-w-0">
          <h1 class="text-lg font-bold text-gray-900 dark:text-zinc-100 tracking-tight">Your credits</h1>
          <p v-if="phase === 'ready'" class="text-xs text-gray-500 dark:text-zinc-400 truncate" data-test="balance-subject">
            {{ balance.service }} · <span class="font-mono">{{ balance.user }}</span> in <span class="font-mono">{{ balance.org }}</span>
          </p>
        </div>
      </header>

      <div v-if="phase === 'loading'" class="flex justify-center py-24" data-test="balance-loading">
        <svg class="animate-spin h-5 w-5 text-gray-400 dark:text-zinc-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
      </div>

      <section v-else-if="phase !== 'ready'" :class="[CARD, 'p-8 text-center']" role="alert" :data-test="`balance-${phase}`">
        <template v-if="phase === 'missing'">
          <h2 class="text-sm font-bold text-gray-900 dark:text-zinc-100">Open this page from your app</h2>
          <p class="mt-1.5 text-xs text-gray-500 dark:text-zinc-400">Your credits are shown through a link from the service you use, such as mesher.</p>
        </template>
        <template v-else-if="phase === 'expired'">
          <h2 class="text-sm font-bold text-gray-900 dark:text-zinc-100">This link has expired</h2>
          <p class="mt-1.5 text-xs text-gray-500 dark:text-zinc-400">Links to your credits work for an hour. Open your credits again from the service you use.</p>
        </template>
        <template v-else>
          <h2 class="text-sm font-bold text-gray-900 dark:text-zinc-100">Your credits couldn't be loaded</h2>
          <p class="mt-1.5 text-xs text-rose-600 dark:text-rose-400" data-test="balance-error-message">{{ error }}</p>
        </template>
      </section>

      <div v-else class="flex flex-col gap-4" data-test="balance-page">
        <p
          v-if="!balance.limited"
          class="rounded-xl border border-gray-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 px-4 py-3 text-xs text-gray-500 dark:text-zinc-400"
          data-test="balance-unlimited"
        >
          {{ balance.service }} doesn't limit credits right now, so nothing is being counted against you.
        </p>

        <div class="grid grid-cols-2 gap-3">
          <StatTile label="Balance" :value="formatCredits(balance.balance)" test="balance-total">
            <span class="text-[11px] text-gray-500 dark:text-zinc-400">Unexpired credits</span>
          </StatTile>
          <StatTile label="Available now" :value="formatCredits(balance.available)" test="balance-available">
            <span class="text-[11px] text-gray-500 dark:text-zinc-400">What you can spend, within your org's limit</span>
          </StatTile>
        </div>

        <section :class="[CARD, 'p-4 flex flex-col gap-3']" data-test="balance-monthly">
          <div class="flex items-baseline gap-3">
            <h2 :class="[HEADING, 'grow']">This month</h2>
            <span class="text-[11px] text-gray-500 dark:text-zinc-400">Renews {{ formatDateTime(balance.renewsAt) }}</span>
          </div>
          <div class="flex items-baseline gap-1.5 tabular-nums">
            <span class="text-2xl font-black text-gray-900 dark:text-zinc-100">{{ formatCredits(balance.monthly.remaining) }}</span>
            <span class="text-xs text-gray-500 dark:text-zinc-400">left of {{ formatCredits(balance.monthly.limit) }} · {{ formatCredits(balance.monthly.used) }} used</span>
          </div>
          <div class="h-1.5 rounded-full bg-gray-100 dark:bg-zinc-800 overflow-hidden">
            <div
              class="h-full rounded-full"
              :class="monthlyPercent >= 100 ? 'bg-rose-500' : 'bg-gray-900 dark:bg-zinc-100'"
              :style="{ width: `${monthlyPercent}%` }"
              data-test="balance-monthly-bar"
            />
          </div>
          <p class="text-[11px] text-gray-500 dark:text-zinc-400" data-test="balance-org">
            Your org has {{ formatCredits(balance.orgUsage.remaining) }} of its {{ formatCredits(balance.orgUsage.limit) }} monthly credits left.
            Unused monthly credits don't carry over.
          </p>
        </section>

        <section :class="CARD" data-test="balance-grants">
          <h2 :class="[HEADING, 'px-4 pt-4 pb-2']">Grants</h2>
          <p v-if="balance.grants.length === 0" class="px-4 pb-4 text-xs text-gray-400 dark:text-zinc-500" data-test="balance-grants-empty">
            No extra credits right now.
          </p>
          <ul v-else class="divide-y divide-gray-100 dark:divide-zinc-800 border-t border-gray-100 dark:border-zinc-800">
            <li v-for="g in balance.grants" :key="g.id" class="flex items-center gap-3 px-4 py-2.5 text-xs" :data-test="`grant-${g.id}`">
              <span class="grow min-w-0">
                <span class="block font-semibold text-gray-900 dark:text-zinc-100 tabular-nums">{{ formatCredits(g.remaining) }} of {{ formatCredits(g.credits) }} left</span>
                <span class="block text-[10px] text-gray-400 dark:text-zinc-500">Granted {{ formatDate(g.createdAt) }}</span>
              </span>
              <span class="shrink-0 text-right text-gray-500 dark:text-zinc-400">Expires {{ formatDateTime(g.expiresAt) }}</span>
            </li>
          </ul>
        </section>

        <section :class="[CARD, 'overflow-hidden']" data-test="balance-history">
          <div :class="[HISTORY_ROW, HEADING, 'pt-4 pb-2']">
            <h2>History</h2>
            <span v-if="entries.length" class="text-right">Change</span>
            <span v-if="entries.length" class="text-right">Balance</span>
          </div>
          <p v-if="entries.length === 0" class="px-4 pb-4 text-xs text-gray-400 dark:text-zinc-500" data-test="balance-history-empty">Nothing yet.</p>
          <ul v-else class="divide-y divide-gray-100 dark:divide-zinc-800 border-t border-gray-100 dark:border-zinc-800">
            <li
              v-for="e in entries"
              :key="e.id"
              :class="[HISTORY_ROW, 'py-2.5 text-xs']"
              :data-test="`entry-${e.id}`"
            >
              <span class="min-w-0">
                <span class="block truncate font-semibold text-gray-900 dark:text-zinc-100">
                  {{ KIND_LABEL[e.kind] || e.kind }}<template v-if="e.operation && SHOWS_OPERATION.has(e.kind)"> · <span class="font-normal font-mono">{{ e.operation }}</span></template>
                </span>
                <span class="block text-[10px] text-gray-400 dark:text-zinc-500">{{ formatDateTime(e.createdAt) }}</span>
              </span>
              <span class="text-right tabular-nums font-semibold" :class="e.amount < 0 ? 'text-rose-600 dark:text-rose-400' : 'text-emerald-600 dark:text-emerald-400'">{{ signed(e.amount) }}</span>
              <span class="text-right tabular-nums text-gray-500 dark:text-zinc-400">{{ formatCredits(e.balanceAfter) }}</span>
            </li>
          </ul>
          <div v-if="nextCursor" class="px-4 py-3 border-t border-gray-100 dark:border-zinc-800">
            <button type="button" :class="SECONDARY" :disabled="loadingMore" data-test="balance-more" @click="loadMore">Load more</button>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>
