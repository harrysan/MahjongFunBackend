<script setup lang="ts">
import { ref } from 'vue'
import { useGameStore } from '../stores/game'
import { cleanPrefs, toSettings, type Prefs } from '../settings'
import SettingsForm from '../components/SettingsForm.vue'
import { canInstall, installApp } from '../pwa'

const game = useGameStore()
const NAME_KEY = 'mahjong.name'
const SETTINGS_KEY = 'mahjong.settings'

function read(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function write(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {
    // storage unavailable; the choice is just not remembered
  }
}

function loadPrefs(): Prefs {
  try {
    return cleanPrefs(JSON.parse(read(SETTINGS_KEY) ?? 'null'))
  } catch {
    return cleanPrefs(null)
  }
}

const name = ref(read(NAME_KEY) ?? '')
const prefs = ref<Prefs>(loadPrefs())
// An invite link looks like ?room=ABCDE.
const code = ref(new URL(location.href).searchParams.get('room')?.toUpperCase() ?? '')

function playerName(): string {
  const n = name.value.trim() || 'Pemain'
  write(NAME_KEY, n)
  return n
}

function savePrefs() {
  write(SETTINGS_KEY, JSON.stringify(prefs.value))
}

function playSolo() {
  savePrefs()
  game.startSolo(playerName(), toSettings(prefs.value))
}

function createRoom() {
  savePrefs()
  game.createRoom(playerName(), toSettings(prefs.value))
}

function joinRoom() {
  if (code.value.trim()) game.joinRoom(playerName(), code.value)
}
</script>

<template>
  <main class="lobby">
    <h1>Mahjong <span>Fun</span></h1>
    <p class="tagline">4 pemain, tanpa skor. Siapa yang hu duluan, dia yang menang.</p>

    <div class="card">
      <label for="name">Nama kamu</label>
      <input id="name" v-model="name" maxlength="20" placeholder="Pemain" autocomplete="nickname" />

      <p v-if="game.lobbyError" class="lobby-error" role="alert">{{ game.lobbyError }}</p>

      <form v-if="code" class="join highlight" @submit.prevent="joinRoom">
        <span class="join-title">Kamu diundang ke room <b>{{ code }}</b></span>
        <button type="submit" class="primary">Gabung</button>
      </form>

      <button type="button" class="primary" @click="playSolo">Main Solo (vs 3 Bot)</button>
      <button type="button" class="secondary" @click="createRoom">Buat Room untuk Teman</button>

      <form class="join" @submit.prevent="joinRoom">
        <label for="code">Punya kode room?</label>
        <div class="join-row">
          <input
            id="code"
            v-model="code"
            maxlength="32"
            placeholder="Contoh: K7QX2"
            autocomplete="off"
            autocapitalize="characters"
            spellcheck="false"
          />
          <button type="submit" class="secondary" :disabled="!code.trim()">Gabung</button>
        </div>
      </form>

      <details class="settings" open>
        <summary>Pengaturan (untuk room yang kamu buat)</summary>
        <SettingsForm v-model="prefs" id-prefix="lobby" />
      </details>
    </div>

    <button v-if="canInstall" type="button" class="install" @click="installApp">
      📲 Pasang sebagai aplikasi
    </button>
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
  max-width: 380px;
  padding: 20px;
  border-radius: 16px;
  background: rgba(0, 0, 0, 0.25);
  text-align: left;
}
label { color: #f3efe2; font-weight: 600; }
input {
  min-width: 0;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid #2c6b4f;
  background: #fffdf6;
  color: #222;
  font: inherit;
  font-size: 16px;
}
#code { flex: 1; text-transform: uppercase; letter-spacing: 2px; }
.lobby-error {
  margin: 0;
  padding: 8px 12px;
  border-radius: 8px;
  background: #b3261e;
  color: #fff;
  font-size: 14px;
}
.primary, .secondary {
  padding: 12px;
  border: none;
  border-radius: 10px;
  font: inherit;
  font-weight: 700;
  font-size: 16px;
  cursor: pointer;
}
.primary { background: #ffcf3f; color: #3a2a00; box-shadow: 0 3px 0 #b8901c; }
.primary:active { transform: translateY(2px); box-shadow: 0 1px 0 #b8901c; }
.secondary { background: rgba(255, 255, 255, 0.14); color: #fff8e0; }
.secondary:disabled { opacity: 0.5; cursor: default; }
.join { display: flex; flex-direction: column; gap: 6px; margin-top: 4px; }
.join-row { display: flex; gap: 8px; }
.join.highlight {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin: 0;
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(255, 207, 63, 0.15);
  border: 1px solid rgba(255, 207, 63, 0.5);
}
.join-title { color: #fff8e0; }
.join.highlight .primary { padding: 8px 16px; }
.settings {
  margin-top: 4px;
  padding: 10px 12px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 12px;
}
.settings summary { cursor: pointer; color: #ffcf3f; font-weight: 700; }
.install {
  padding: 10px 18px;
  border: 1px solid rgba(255, 207, 63, 0.6);
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.2);
  color: #fff8e0;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}
.settings[open] summary { margin-bottom: 10px; }
</style>
