// Copyright 2026 Contextual, Inc.
// This notice may not be modified or removed.
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, expect, it } from 'vitest'
import { formatCredits, formatDateTime, formatLimit, limitText, parseLimit } from '../src/utils/credits'

describe('credit helpers', () => {
  it('parseLimit reads blank as uncapped and rejects non-whole numbers', () => {
    expect(parseLimit('')).toBeNull()
    expect(parseLimit('  ')).toBeNull()
    expect(parseLimit(undefined)).toBeNull()
    expect(parseLimit(' 42 ')).toBe(42)
    expect(parseLimit('0')).toBe(0)
    expect(parseLimit('-1')).toBeUndefined()
    expect(parseLimit('1.5')).toBeUndefined()
    expect(parseLimit('x')).toBeUndefined()
  })

  it('limitText shows uncapped as blank', () => {
    expect(limitText(null)).toBe('')
    expect(limitText(undefined)).toBe('')
    expect(limitText(0)).toBe('0')
  })

  it('formats credits, limits and times', () => {
    expect(formatCredits(1234567)).toBe('1,234,567')
    expect(formatLimit(null)).toBe('Uncapped')
    expect(formatLimit(undefined)).toBe('Uncapped')
    expect(formatLimit(1000)).toBe('1,000')
    expect(formatDateTime('2026-11-21T12:00:00Z')).toContain('2026')
  })
})
