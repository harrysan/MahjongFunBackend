<script setup lang="ts">
import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useGameStore } from '../stores/game'
import { describeSettings, fromSettings, toSettings, type Prefs } from '../settings'
import { WIND_NAMES } from '../tiles'
import SettingsForm from '../components/SettingsForm.vue'

const game = useGameStore()
const { room, isHost, mySeat, error } = storeToRefs(game)

const inviteLink = computed(() => {
  const url = new URL(location.href)
  url.search = ''
  url.searchParams.set('room', room.value!.room)
  return url.toString()
})
const emptySeats = computed(() => room.value!.seats.filter((s) => s.kind === 'empty').length)
const humans = computed(() => room.value!.seats.filter((s) => s.kind === 'human').length)
const prefs = computed(() => fromSettings(room.value!.settings))

function changeSettings(p: Prefs) {
  game.updateSettings(toSettings(p))
}

const copied = ref<'link' | 'code' | null>(null)
async function copy(what: 'link' | 'code') {
  const text = what === 'link' ? inviteLink.value : room.value!.room
  try {
    await navigator.clipboard.writeText(text)
    copied.value = what
    setTimeout(() => (copied.value = null), 2000)
  } catch {
    window.prompt('Salin teks ini:', text)
  }
}
</script>

<template>
  <main v-if="room" class="waiting">
    <header class="bar">
      <button type="button" class="link" @click="game.leave()">← Keluar</button>
    </header>

    <section class="card">
      <h1>Ruang Tunggu</h1>
      <div class="code-box">
        <span class="code-label">Kode room</span>
        <span class="code">{{ room.room }}</span>
      </div>
      <div class="invite">
        <button type="button" class="secondary" @click="copy('link')">
          {{ copied === 'link' ? 'Tautan disalin ✓' : 'Salin tautan undangan' }}
        </button>
        <button type="button" class="secondary" @click="copy('code')">
          {{ copied === 'code' ? 'Kode disalin ✓' : 'Salin kode' }}
        </button>
      </div>
      <p class="help">Bagikan kode atau tautan ini ke teman. Kursi kosong bisa diisi bot.</p>
    </section>

    <section class="card">
      <h2>Pemain ({{ 4 - emptySeats }}/4)</h2>
      <ul class="seats">
        <li v-for="s in room.seats" :key="s.seat" :class="s.kind">
          <span class="wind">{{ WIND_NAMES[s.seat] }}</span>
          <span class="who">
            <template v-if="s.kind === 'empty'"><i>Kursi kosong</i></template>
            <template v-else>
              {{ s.name }}
              <span v-if="s.seat === mySeat" class="tag me">kamu</span>
              <span v-if="s.seat === room.host" class="tag host">host</span>
              <span v-if="s.kind === 'bot'" class="tag">bot</span>
              <span v-if="s.kind === 'human' && !s.connected" class="tag away">terputus</span>
            </template>
          </span>
          <template v-if="isHost">
            <button v-if="s.kind === 'empty'" type="button" class="small" @click="game.addBot(s.seat)">+ Bot</button>
            <button v-else-if="s.kind === 'bot'" type="button" class="small ghost" @click="game.removeBot(s.seat)">
              Hapus
            </button>
          </template>
        </li>
      </ul>
      <button v-if="isHost && emptySeats > 0" type="button" class="secondary" @click="game.fillWithBots()">
        Isi kursi kosong dengan bot
      </button>
    </section>

    <section class="card">
      <h2>Pengaturan</h2>
      <SettingsForm v-if="isHost" :model-value="prefs" id-prefix="room" @update:model-value="changeSettings" />
      <p v-else class="help">{{ describeSettings(room.settings) }}</p>
    </section>

    <div class="start">
      <template v-if="isHost">
        <button type="button" class="primary" :disabled="emptySeats > 0" @click="game.startGame()">
          Mulai Permainan
        </button>
        <p v-if="emptySeats > 0" class="help">Isi semua kursi dulu (pemain atau bot).</p>
        <p v-else-if="humans === 1" class="help">Kamu satu-satunya pemain manusia.</p>
      </template>
      <p v-else class="help">Menunggu host memulai permainan…</p>
    </div>

    <div v-if="error" class="toast" role="alert">{{ error }}</div>
  </main>
</template>

<style scoped>
.waiting {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 8px 16px 32px;
}
.bar { align-self: stretch; color: #e8f1ea; }
.link { background: none; border: none; color: inherit; font: inherit; cursor: pointer; padding: 4px 0; }
.card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
  max-width: 420px;
  padding: 18px 20px;
  border-radius: 16px;
  background: rgba(0, 0, 0, 0.25);
}
h1, h2 { margin: 0; color: #fff8e0; }
h1 { font-size: 24px; }
h2 { font-size: 17px; }
.code-box { display: flex; flex-direction: column; align-items: center; gap: 2px; padding: 8px 0; }
.code-label { color: #9fbcab; font-size: 13px; }
.code { font-size: 40px; font-weight: 800; letter-spacing: 6px; color: #ffcf3f; font-variant-numeric: tabular-nums; }
.invite { display: flex; flex-wrap: wrap; gap: 8px; }
.invite button { flex: 1 1 140px; }
.help { margin: 0; color: #9fbcab; font-size: 13px; text-align: center; }

.seats { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
.seats li {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 44px;
  padding: 6px 10px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.07);
  color: #f3efe2;
}
.seats li.empty { background: rgba(255, 255, 255, 0.03); color: #9fbcab; }
.wind {
  flex: none;
  width: 62px;
  padding: 2px 0;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.3);
  font-size: 12px;
  text-align: center;
}
.who { flex: 1; min-width: 0; overflow-wrap: anywhere; }
.tag { margin-left: 4px; padding: 1px 6px; border-radius: 4px; font-size: 11px; background: rgba(255, 255, 255, 0.15); }
.tag.me { background: #1f8a5b; }
.tag.host { background: #c62828; }
.tag.away { background: #6b4d00; }

.primary, .secondary, .small {
  border: none;
  border-radius: 10px;
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}
.primary { padding: 14px 28px; font-size: 18px; background: #ffcf3f; color: #3a2a00; box-shadow: 0 3px 0 #b8901c; }
.primary:active:not(:disabled) { transform: translateY(2px); box-shadow: 0 1px 0 #b8901c; }
.primary:disabled { opacity: 0.5; cursor: default; }
.secondary { padding: 10px 14px; background: rgba(255, 255, 255, 0.14); color: #fff8e0; }
.small { flex: none; padding: 6px 12px; font-size: 13px; background: #3b6fd8; color: #fff; }
.small.ghost { background: rgba(255, 255, 255, 0.12); }
.start { display: flex; flex-direction: column; align-items: center; gap: 8px; margin-top: 4px; }

.toast {
  position: fixed;
  top: 16px;
  left: 50%;
  transform: translateX(-50%);
  max-width: calc(100vw - 32px);
  padding: 10px 16px;
  border-radius: 10px;
  background: #b3261e;
  color: #fff;
  font-weight: 600;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
  z-index: 20;
}
</style>
