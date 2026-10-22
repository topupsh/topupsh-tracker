<!--
  Copyright 2026 Contextual, Inc.
  This notice may not be modified or removed.
  SPDX-License-Identifier: AGPL-3.0-only
-->

<script setup>
// The signed-in landing page: a placeholder until the tracker has features.
import { computed, inject } from 'vue'
import BrandLogo from '../components/BrandLogo.vue'

const { user } = inject('currentUser')

const greeting = computed(() => {
  const name = user.value?.name || user.value?.email
  return name ? `Welcome, ${name}` : 'Welcome'
})
</script>

<template>
  <div class="flex flex-col gap-4" data-test="home">
    <header class="px-4">
      <h1 class="text-lg font-bold text-gray-900 dark:text-zinc-100 tracking-tight" data-test="home-greeting">{{ greeting }}</h1>
      <p class="text-xs text-gray-500 dark:text-zinc-400">topupsh tracker</p>
    </header>

    <section
      class="mx-4 flex flex-col items-center justify-center gap-3 rounded-xl border border-gray-100 dark:border-zinc-800 bg-white dark:bg-zinc-900 px-6 py-16 text-center"
      data-test="home-empty"
    >
      <BrandLogo class="w-8 h-8 text-gray-400 dark:text-zinc-500" />
      <h2 class="text-[11px] font-black uppercase tracking-widest text-gray-900 dark:text-zinc-100">Nothing here yet</h2>
      <p class="max-w-sm text-xs text-gray-500 dark:text-zinc-400">Tracking features are on the way. This page will fill in as they land.</p>
    </section>
  </div>
</template>
