import type { Settings } from './types'

// The choices offered in the UI. The server accepts any value within its
// limits (see server/internal/room/config.go); these are just the presets.

export const BOT_SPEEDS = [
  { label: 'Kilat (0–0,2 detik)', min: 0, max: 200 },
  { label: 'Cepat (0,2–0,6 detik)', min: 200, max: 600 },
  { label: 'Normal (0,5–1,5 detik)', min: 500, max: 1500 },
  { label: 'Santai (1,5–3 detik)', min: 1500, max: 3000 },
  { label: 'Lambat (3–5 detik)', min: 3000, max: 5000 },
]
export const TURN_OPTIONS = [10, 15, 20, 30, 45, 60, 0]
export const CLAIM_OPTIONS = [5, 10, 15, 20, 0]

/** What the settings form edits: indexes/values of the presets above. */
export interface Prefs {
  botSpeed: number
  turn: number
  claim: number
}

export const DEFAULT_PREFS: Prefs = { botSpeed: 2, turn: 30, claim: 10 }

export function secondsLabel(n: number): string {
  return n === 0 ? 'Tanpa batas' : `${n} detik`
}

export function toSettings(p: Prefs): Settings {
  const speed = BOT_SPEEDS[p.botSpeed] ?? BOT_SPEEDS[DEFAULT_PREFS.botSpeed]
  return {
    bot_delay_min_ms: speed.min,
    bot_delay_max_ms: speed.max,
    turn_seconds: p.turn,
    claim_seconds: p.claim,
  }
}

export function fromSettings(s: Settings): Prefs {
  const i = BOT_SPEEDS.findIndex((b) => b.min === s.bot_delay_min_ms && b.max === s.bot_delay_max_ms)
  return {
    botSpeed: i >= 0 ? i : DEFAULT_PREFS.botSpeed,
    turn: s.turn_seconds,
    claim: s.claim_seconds,
  }
}

/** Keep only known presets from something loaded from storage. */
export function cleanPrefs(p: Partial<Prefs> | null | undefined): Prefs {
  return {
    botSpeed: p?.botSpeed !== undefined && BOT_SPEEDS[p.botSpeed] ? p.botSpeed : DEFAULT_PREFS.botSpeed,
    turn: p?.turn !== undefined && TURN_OPTIONS.includes(p.turn) ? p.turn : DEFAULT_PREFS.turn,
    claim: p?.claim !== undefined && CLAIM_OPTIONS.includes(p.claim) ? p.claim : DEFAULT_PREFS.claim,
  }
}

/** Human-readable one-line summary, for players who cannot change the settings. */
export function describeSettings(s: Settings): string {
  const p = fromSettings(s)
  const bot = BOT_SPEEDS[p.botSpeed].label.split(' (')[0]
  return `Bot: ${bot} · Giliran: ${secondsLabel(p.turn)} · Klaim: ${secondsLabel(p.claim)}`
}
