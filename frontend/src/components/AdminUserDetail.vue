<!--
  Copyright 2026 Contextual, Inc.
  This notice may not be modified or removed.
  SPDX-License-Identifier: AGPL-3.0-only
-->

<script setup>
// One user as root sees them.
import { onMounted, ref } from 'vue'
import { fetchAdminUser } from '../api'

const props = defineProps({
  // The user's base62 ID.
  id: { type: String, required: true },
})
const emit = defineEmits(['back', 'error'])

const SECONDARY =
  'rounded-lg border border-gray-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 px-3 py-2 text-[11px] font-black uppercase tracking-widest text-gray-900 dark:text-zinc-100 hover:border-gray-300 dark:hover:border-zinc-700 transition-colors'

const user = ref(null)
const missing = ref(false)
const failed = ref(false)

function formatDate(iso) {
  return new Date(iso).toLocaleDateString(undefined, { dateStyle: 'medium' })
}

onMounted(async () => {
  try {
    user.value = await fetchAdminUser(props.id)
  } catch (err) {
    if (err.status === 404) {
      missing.value = true
      return
    }
    failed.value = true
    emit('error', err)
  }
})
</script>

<template>
  <div class="flex flex-col gap-4" data-test="admin-user">
    <div>
      <button type="button" :class="SECONDARY" data-test="admin-user-back" @click="emit('back')">← All users</button>
    </div>

    <p v-if="missing" class="rounded-xl border border-gray-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 px-4 py-6 text-xs text-gray-500 dark:text-zinc-400" data-test="admin-user-missing">
      No user has the ID <span class="font-mono">{{ id }}</span>.
    </p>

    <p v-else-if="failed" class="rounded-xl border border-gray-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 px-4 py-6 text-xs text-gray-500 dark:text-zinc-400" data-test="admin-user-failed">
      This user could not be loaded.
    </p>

    <template v-else-if="user">
      <header class="flex flex-wrap items-center gap-3 rounded-xl border border-gray-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 p-4">
        <img v-if="user.picture" :src="user.picture" alt="" class="w-10 h-10 rounded-full shrink-0" referrerpolicy="no-referrer" />
        <div class="grow min-w-0">
          <h2 class="text-sm font-bold text-gray-900 dark:text-zinc-100 truncate" data-test="admin-user-name">{{ user.name || user.email }}</h2>
          <p class="text-xs text-gray-500 dark:text-zinc-400 truncate">{{ user.email }}</p>
          <p class="text-[10px] text-gray-400 dark:text-zinc-500">
            <span class="font-mono" data-test="admin-user-id">{{ user.id }}</span>
            <template v-if="user.provider"> · {{ user.provider }}</template> · joined {{ formatDate(user.createdAt) }}
          </p>
        </div>
      </header>
    </template>

    <p v-else class="px-1 py-6 text-[10px] font-black text-gray-300 dark:text-zinc-500 animate-pulse" data-test="admin-user-loading">Loading...</p>
  </div>
</template>
