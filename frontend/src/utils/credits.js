// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

// Class lists the credit admin screens share.
export const INPUT =
  'w-full rounded-lg border border-gray-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 px-3 py-2 text-sm text-gray-900 dark:text-zinc-100 placeholder:text-gray-400 dark:placeholder:text-zinc-500 focus:border-gray-400 dark:focus:border-zinc-500 focus:outline-none'
export const LABEL = 'block text-[11px] font-bold text-gray-500 dark:text-zinc-400 mb-1'
export const PRIMARY =
  'rounded-lg bg-black dark:bg-white px-4 py-2 text-[11px] font-black uppercase tracking-widest text-white dark:text-zinc-900 hover:bg-gray-800 dark:hover:bg-zinc-200 disabled:opacity-50 transition-colors'
export const SECONDARY =
  'rounded-lg border border-gray-200 dark:border-zinc-800 bg-white dark:bg-zinc-900 px-3 py-2 text-[11px] font-black uppercase tracking-widest text-gray-900 dark:text-zinc-100 hover:border-gray-300 dark:hover:border-zinc-700 disabled:opacity-50 transition-colors'
export const CARD = 'bg-white dark:bg-zinc-900 border border-gray-200 dark:border-zinc-800 rounded-xl'
export const HEADING = 'text-[10px] font-black uppercase tracking-widest text-gray-400 dark:text-zinc-500'

// parseLimit reads a limit input: blank is null (uncapped), a whole number
// of 0 or more is that number, anything else is undefined (invalid).
export function parseLimit(text) {
  const s = String(text ?? '').trim()
  if (s === '') return null
  return /^\d+$/.test(s) ? Number(s) : undefined
}

// limitText is a limit as an input value: null (uncapped) is blank.
export function limitText(limit) {
  return limit === null || limit === undefined ? '' : String(limit)
}

export function formatCredits(n) {
  return Number(n).toLocaleString('en-US')
}

// formatLimit shows a user limit, null meaning uncapped.
export function formatLimit(limit) {
  return limit === null || limit === undefined ? 'Uncapped' : formatCredits(limit)
}

export function formatDateTime(iso) {
  return new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}
