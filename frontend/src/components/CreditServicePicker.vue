<!--
  Copyright 2026 Contextual, Inc.
  This notice may not be modified or removed.
  SPDX-License-Identifier: AGPL-3.0-only
-->

<script setup>
// Picks the credit service a screen works on; the first one until the
// admin picks another.
import { onMounted, ref } from 'vue'
import { listServices } from '../api'
import { CARD, INPUT, LABEL } from '../utils/credits'

const model = defineModel({ type: String, default: '' })
const emit = defineEmits(['error'])

const names = ref([])
const loading = ref(true)

onMounted(async () => {
  try {
    names.value = (await listServices()).map((s) => s.name)
    if (!names.value.includes(model.value)) model.value = names.value[0] || ''
  } catch (err) {
    emit('error', err)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div data-test="service-picker">
    <p v-if="loading" class="py-2 text-[10px] font-black text-gray-300 dark:text-zinc-500 animate-pulse">Loading...</p>
    <p v-else-if="names.length === 0" :class="[CARD, 'px-4 py-6 text-xs text-gray-400 dark:text-zinc-500']" data-test="service-picker-empty">
      No services yet. Create one under Services.
    </p>
    <div v-else class="max-w-xs">
      <label for="service-picker" :class="LABEL">Service</label>
      <select id="service-picker" v-model="model" :class="INPUT" data-test="service-picker-select">
        <option v-for="n in names" :key="n" :value="n">{{ n }}</option>
      </select>
    </div>
  </div>
</template>
