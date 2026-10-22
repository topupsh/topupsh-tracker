// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { createWebHistory } from 'vue-router'
import { createTrackerApp } from './app'
import { registerServiceWorker } from './composables/usePwaUpdate'

const basePath = window.__TOPUPSH_BASE_PATH__ || ''

const { app } = createTrackerApp({
  history: createWebHistory(basePath || '/'),
  platform: 'web',
})
app.mount('#app')

// Browser only: installs the PWA and downloads new versions in the background.
registerServiceWorker({ basePath })
