<script setup lang="ts">
import { useGameStore } from './stores/game'
import { useConnectionStore } from './stores/connection'
import Lobby from './views/Lobby.vue'
import WaitingRoom from './views/WaitingRoom.vue'
import Table from './views/Table.vue'

const game = useGameStore()
const conn = useConnectionStore()

// After a refresh, go straight back to the game this tab was in.
game.resumeSession()
</script>

<template>
  <Lobby v-if="!game.room && conn.status === 'idle'" />
  <main v-else-if="!game.room" class="connecting">
    <p>Menghubungkan…</p>
    <button type="button" class="ghost" @click="game.goToLobby(null)">Batal</button>
  </main>
  <WaitingRoom v-else-if="game.room.status === 'waiting'" />
  <Table v-else />

  <div v-if="game.room && game.replaced" class="overlay">
    <div class="dialog" role="alertdialog" aria-labelledby="moved-title">
      <h2 id="moved-title">Dibuka di tempat lain</h2>
      <p>Permainan ini sedang dibuka di tab atau perangkat lain.</p>
      <div class="actions">
        <button type="button" @click="game.reconnectNow()">Main di sini</button>
        <button type="button" class="ghost" @click="game.goToLobby(null)">Ke lobby</button>
      </div>
    </div>
  </div>
  <div v-else-if="game.room && game.reconnecting" class="overlay">
    <div class="dialog" role="alertdialog" aria-labelledby="lost-title">
      <span class="spinner" aria-hidden="true" />
      <h2 id="lost-title">Koneksi terputus</h2>
      <p>Menyambung kembali… Sementara itu bot memainkan giliranmu.</p>
      <div class="actions">
        <button type="button" @click="game.reconnectNow()">Coba sekarang</button>
        <button type="button" class="ghost" @click="game.goToLobby(null)">Ke lobby</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.connecting {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
}
.overlay {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  background: rgba(0, 0, 0, 0.5);
  z-index: 30;
}
.dialog {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  width: 100%;
  max-width: 380px;
  padding: 24px;
  border-radius: 16px;
  background: #fffdf6;
  color: #222;
  text-align: center;
}
.dialog h2, .dialog p { margin: 0; }
.actions { display: flex; flex-wrap: wrap; gap: 10px; justify-content: center; }
button {
  padding: 10px 18px;
  border: none;
  border-radius: 10px;
  background: #1f8a5b;
  color: #fff;
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}
button.ghost { background: #e3ddcb; color: #333; }
.spinner {
  width: 28px;
  height: 28px;
  border: 3px solid #e3ddcb;
  border-top-color: #1f8a5b;
  border-radius: 50%;
  animation: spin 0.9s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .spinner { animation-duration: 3s; } }
</style>
