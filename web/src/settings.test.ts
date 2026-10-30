import { describe, expect, it } from 'vitest'
import { BOT_SPEEDS, DEFAULT_PREFS, cleanPrefs, describeSettings, fromSettings, secondsLabel, toSettings } from './settings'

describe('settings', () => {
  it('round-trips every preset through the wire format', () => {
    for (let botSpeed = 0; botSpeed < BOT_SPEEDS.length; botSpeed++) {
      const p = { botSpeed, turn: 15, claim: 0 }
      expect(fromSettings(toSettings(p))).toEqual(p)
    }
  })

  it('falls back to the default bot speed for unknown server values', () => {
    const p = fromSettings({ bot_delay_min_ms: 1, bot_delay_max_ms: 2, turn_seconds: 30, claim_seconds: 10 })
    expect(p.botSpeed).toBe(DEFAULT_PREFS.botSpeed)
  })

  it('cleans stored preferences', () => {
    expect(cleanPrefs(null)).toEqual(DEFAULT_PREFS)
    expect(cleanPrefs({ botSpeed: 99, turn: 7, claim: 5 })).toEqual({ ...DEFAULT_PREFS, claim: 5 })
    expect(cleanPrefs({ botSpeed: 0, turn: 0, claim: 0 })).toEqual({ botSpeed: 0, turn: 0, claim: 0 })
  })

  it('describes settings in Indonesian', () => {
    expect(secondsLabel(0)).toBe('Tanpa batas')
    expect(describeSettings(toSettings({ botSpeed: 0, turn: 30, claim: 0 }))).toBe(
      'Bot: Kilat · Giliran: 30 detik · Klaim: Tanpa batas',
    )
  })
})
