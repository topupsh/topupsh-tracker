<!--
  Copyright 2026 Contextual, Inc.
  This notice may not be modified or removed.
  SPDX-License-Identifier: AGPL-3.0-only
-->

<script setup>
// The ledger of every service, newest first, filtered by service, org and
// user.
import { onMounted, reactive, ref } from 'vue'
import { listLedger, listServices } from '../api'
import { CARD, HEADING, INPUT, LABEL, SECONDARY, formatCredits, formatDateTime } from '../utils/credits'

const emit = defineEmits(['error'])

// Columns: when, who, kind, amount, balance after; actor from md up.
const ROW = 'grid grid-cols-[minmax(0,1fr)_4rem_4rem] md:grid-cols-[9rem_minmax(0,1fr)_5rem_5rem_5rem_7rem] items-center gap-3 px-4'
const KIND = {
  spend: 'bg-gray-100 dark:bg-zinc-800 text-gray-600 dark:text-zinc-300',
  refund: 'bg-sky-50 dark:bg-sky-500/10 text-sky-700 dark:text-sky-400',
  grant: 'bg-emerald-50 dark:bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
  renewal: 'bg-violet-50 dark:bg-violet-500/10 text-violet-700 dark:text-violet-400',
  limit: 'bg-amber-50 dark:bg-amber-500/10 text-amber-700 dark:text-amber-400',
  expiry: 'bg-rose-50 dark:bg-rose-500/10 text-rose-600 dark:text-rose-400',
}

const services = ref([])
const filters = reactive({ service: '', org: '', user: '' })
// The filters the shown entries answer.
const applied = ref({ service: '', org: '', user: '' })
const entries = ref([])
const nextCursor = ref('')
const loading = ref(true)
const loadingMore = ref(false)

async function search() {
  loading.value = true
  applied.value = { service: filters.service, org: filters.org.trim(), user: filters.user.trim() }
  try {
    const page = await listLedger(applied.value)
    entries.value = page.entries
    nextCursor.value = page.nextCursor || ''
  } catch (err) {
    emit('error', err)
  } finally {
    loading.value = false
  }
}

async function loadMore() {
  loadingMore.value = true
  try {
    const page = await listLedger({ ...applied.value, cursor: nextCursor.value })
    entries.value.push(...page.entries)
    nextCursor.value = page.nextCursor || ''
  } catch (err) {
    emit('error', err)
  } finally {
    loadingMore.value = false
  }
}

function signed(n) {
  return n > 0 ? `+${formatCredits(n)}` : formatCredits(n)
}

onMounted(async () => {
  try {
    services.value = (await listServices()).map((s) => s.name)
  } catch (err) {
    emit('error', err)
  }
  await search()
})
</script>

<template>
  <div class="flex flex-col gap-4" data-test="credit-ledger">
    <form :class="[CARD, 'p-4 grid grid-cols-1 sm:grid-cols-[10rem_minmax(0,1fr)_minmax(0,1fr)_auto] sm:items-end gap-3']" role="search" data-test="ledger-filters" @submit.prevent="search">
      <div>
        <label for="ledger-service" :class="LABEL">Service</label>
        <select id="ledger-service" v-model="filters.service" :class="INPUT" data-test="ledger-service">
          <option value="">All services</option>
          <option v-for="n in services" :key="n" :value="n">{{ n }}</option>
        </select>
      </div>
      <div>
        <label for="ledger-org" :class="LABEL">Org</label>
        <input id="ledger-org" v-model="filters.org" placeholder="Any org" :class="INPUT" data-test="ledger-org" />
      </div>
      <div>
        <label for="ledger-user" :class="LABEL">User</label>
        <input id="ledger-user" v-model="filters.user" :disabled="!filters.org.trim()" :placeholder="filters.org.trim() ? 'Any user' : 'Set an org first'" :class="[INPUT, 'disabled:opacity-50']" data-test="ledger-user" />
      </div>
      <button type="submit" :class="SECONDARY" :disabled="loading" data-test="ledger-search">Filter</button>
    </form>

    <section :class="[CARD, 'overflow-hidden']">
      <div :class="[ROW, HEADING, 'py-2.5 border-b border-gray-100 dark:border-zinc-800']">
        <span class="hidden md:block">When</span>
        <span>Who</span>
        <span class="hidden md:block">Kind</span>
        <span class="text-right">Amount</span>
        <span class="text-right">Balance</span>
        <span class="hidden md:block">By</span>
      </div>
      <p v-if="loading" class="px-4 py-6 text-[10px] font-black text-gray-300 dark:text-zinc-500 animate-pulse" data-test="ledger-loading">Loading...</p>
      <p v-else-if="entries.length === 0" class="px-4 py-6 text-xs text-gray-400 dark:text-zinc-500" data-test="ledger-empty">No ledger entries match.</p>
      <ul v-else class="divide-y divide-gray-100 dark:divide-zinc-800">
        <li v-for="e in entries" :key="e.id" :class="[ROW, 'py-2.5 text-xs']" :data-test="`ledger-${e.id}`">
          <span class="hidden md:block text-gray-500 dark:text-zinc-400">{{ formatDateTime(e.createdAt) }}</span>
          <span class="min-w-0">
            <span class="block truncate font-mono font-semibold text-gray-900 dark:text-zinc-100">{{ e.user }}</span>
            <span class="block truncate text-[10px] text-gray-400 dark:text-zinc-500">
              <span class="font-mono">{{ e.org }}</span> · {{ e.service }}<template v-if="e.operation"> · {{ e.operation }}</template>
            </span>
            <span class="block md:hidden truncate text-[10px] text-gray-400 dark:text-zinc-500">{{ e.kind }} · {{ formatDateTime(e.createdAt) }}</span>
          </span>
          <span class="hidden md:block">
            <span :class="['rounded px-1.5 py-0.5 text-[9px] font-black uppercase tracking-widest', KIND[e.kind] || KIND.spend]">{{ e.kind }}</span>
          </span>
          <span class="text-right tabular-nums font-semibold" :class="e.amount < 0 ? 'text-rose-600 dark:text-rose-400' : 'text-emerald-600 dark:text-emerald-400'">{{ signed(e.amount) }}</span>
          <span class="text-right tabular-nums text-gray-900 dark:text-zinc-100">{{ formatCredits(e.balanceAfter) }}</span>
          <span class="hidden md:block truncate text-gray-500 dark:text-zinc-400">{{ e.actor }}</span>
        </li>
      </ul>
      <div v-if="nextCursor && !loading" class="px-4 py-3 border-t border-gray-100 dark:border-zinc-800">
        <button type="button" :class="SECONDARY" :disabled="loadingMore" data-test="ledger-more" @click="loadMore">Load more</button>
      </div>
    </section>
  </div>
</template>
