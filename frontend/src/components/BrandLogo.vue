<!--
  Copyright 2026 Contextual, Inc.
  This notice may not be modified or removed.
  SPDX-License-Identifier: AGPL-3.0-only
-->

<script setup>
// The topupsh tracker mark (see public/favicon.svg): a rising line inside a
// ring, a balance being tracked.
defineProps({
  // tile draws the mark white on a black square, like the favicon.
  tile: { type: Boolean, default: false },
})
</script>

<template>
  <span
    :class="['inline-flex items-center justify-center shrink-0', tile ? 'bg-black dark:bg-white text-white dark:text-zinc-900 rounded-lg' : 'text-gray-900 dark:text-zinc-100']"
    data-test="brand-logo"
  >
    <svg
      viewBox="0 0 24 24"
      :class="tile ? 'w-3/4 h-3/4' : 'w-full h-full'"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="9" />
      <path d="m7 15 3-3 2.5 2 4.5-5" />
      <path d="M14 9h3v3" />
    </svg>
  </span>
</template>
