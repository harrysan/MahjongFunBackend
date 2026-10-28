<script setup lang="ts">
import { ref } from 'vue'
import { useGameStore } from '../stores/game'
import type { Settings } from '../types'

const game = useGameStore()
const NAME_KEY = 'mahjong.name'
const SETTINGS_KEY = 'mahjong.settings'

const BOT_SPEEDS = [
  { label: 'Kilat (0–0,2 detik)', min: 0, max: 200 },
  { label: 'Cepat (0,2–0,6 detik)', min: 200, max: 600 },
  { label: 'Normal (0,5–1,5 detik)', min: 500, max: 1500 },
  { label: 'Santai (1,5–3 detik)', min: 1500, max: 3000 },
  { label: 'Lambat (3–5 detik)', min: 3000, max: 5000 },
]
const TURN_OPTIONS = [10, 15, 20, 30, 45, 60, 0]
const CLAIM_OPTIONS = [5, 10, 15, 20, 0]

function seconds(n: number): string {
  return n === 0 ? 'Tanpa batas' : `${n} detik`
}

function load<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key)
    return raw === null ? fallback : { ...fallback, ...JSON.parse(raw) }
  } catch {
    return fallback
  }
}

function save(key: string, value: unknown) {
  try {
    localStorage.setItem(key, typeof value === 'string' ? value : JSON.stringify(value))
  } catch {
    // storage unavailable; the choice is just not remembered
  }
}

function loadName(): string {
  try {
    return localStorage.getItem(NAME_KEY) ?? ''
  } catch {
    return ''
  }
}

const saved = load(SETTINGS_KEY, { botSpeed: 2, turn: 30, claim: 10 })
const name = ref(loadName())
const botSpeed = ref(BOT_SPEEDS[saved.botSpeed] ? saved.botSpeed : 2)
const turn = ref(TURN_OPTIONS.includes(saved.turn) ? saved.turn : 30)
const claim = ref(CLAIM_OPTIONS.includes(saved.claim) ? saved.claim : 10)

function playSolo() {
  const n = name.value.trim() || 'Pemain'
  save(NAME_KEY, n)
  save(SETTINGS_KEY, { botSpeed: botSpeed.value, turn: turn.value, claim: claim.value })
  const speed = BOT_SPEEDS[botSpeed.value]
  const settings: Settings = {
    bot_delay_min_ms: speed.min,
    bot_delay_max_ms: speed.max,
    turn_seconds: turn.value,
    claim_seconds: claim.value,
  }
  game.startSolo(n, settings)
}
</script>

<template>
  <main class="lobby">
    <h1>Mahjong <span>Fun</span></h1>
    <p class="tagline">4 pemain, tanpa skor. Siapa yang hu duluan, dia yang menang.</p>
    <form class="card" @submit.prevent="playSolo">
      <label for="name">Nama kamu</label>
      <input id="name" v-model="name" maxlength="20" placeholder="Pemain" autocomplete="nickname" />

      <fieldset>
        <legend>Pengaturan</legend>
        <label for="bot-speed">Waktu bot berpikir</label>
        <select id="bot-speed" v-model.number="botSpeed">
          <option v-for="(s, i) in BOT_SPEEDS" :key="i" :value="i">{{ s.label }}</option>
        </select>

        <label for="turn">Batas waktu giliran</label>
        <select id="turn" v-model.number="turn">
          <option v-for="n in TURN_OPTIONS" :key="n" :value="n">{{ seconds(n) }}</option>
        </select>
        <small>Jika habis, tile yang baru diambil dibuang otomatis.</small>

        <label for="claim">Batas waktu klaim (chi/pung/kong/hu)</label>
        <select id="claim" v-model.number="claim">
          <option v-for="n in CLAIM_OPTIONS" :key="n" :value="n">{{ seconds(n) }}</option>
        </select>
        <small>Jika habis, dianggap pass.</small>
      </fieldset>

      <button type="submit">Main Solo (vs 3 Bot)</button>
      <p class="soon">Main bersama teman segera hadir.</p>
    </form>
  </main>
</template>

<style scoped>
.lobby {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding: 24px 16px;
  text-align: center;
}
h1 { margin: 0; font-size: clamp(40px, 9vw, 64px); color: #fff8e0; letter-spacing: 1px; }
h1 span { color: #ffcf3f; }
.tagline { margin: 0; color: #cfe3d6; }
.card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
  max-width: 360px;
  padding: 20px;
  border-radius: 16px;
  background: rgba(0, 0, 0, 0.25);
  text-align: left;
}
label { color: #f3efe2; font-weight: 600; }
fieldset {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 6px 0 0;
  padding: 12px 14px 14px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 12px;
}
legend { padding: 0 6px; color: #ffcf3f; font-weight: 700; }
fieldset label { margin-top: 6px; font-size: 14px; }
small { color: #9fbcab; font-size: 12px; }
input, select {
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid #2c6b4f;
  background: #fffdf6;
  color: #222;
  font: inherit;
  font-size: 16px;
}
button {
  margin-top: 6px;
  padding: 12px;
  border: none;
  border-radius: 10px;
  background: #ffcf3f;
  color: #3a2a00;
  font: inherit;
  font-weight: 700;
  font-size: 17px;
  cursor: pointer;
  box-shadow: 0 3px 0 #b8901c;
}
button:active { transform: translateY(2px); box-shadow: 0 1px 0 #b8901c; }
.soon { margin: 4px 0 0; color: #9fbcab; font-size: 13px; text-align: center; }
</style>
