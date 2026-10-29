<script setup lang="ts">
import { useGameStore } from './stores/game'
import { useConnectionStore } from './stores/connection'
import Lobby from './views/Lobby.vue'
import WaitingRoom from './views/WaitingRoom.vue'
import Table from './views/Table.vue'

const game = useGameStore()
const conn = useConnectionStore()
</script>

<template>
  <Lobby v-if="!game.room && conn.status === 'idle'" />
  <main v-else-if="!game.room" class="connecting">
    <p>Menghubungkan…</p>
    <button type="button" @click="game.leave()">Batal</button>
  </main>
  <WaitingRoom v-else-if="game.room.status === 'waiting'" />
  <Table v-else />

  <div v-if="game.room && conn.status === 'closed'" class="overlay">
    <div class="dialog" role="alertdialog" aria-labelledby="lost-title">
      <h2 id="lost-title">Terputus</h2>
      <p>Koneksi ke server terputus.</p>
      <button type="button" @click="game.leave()">Kembali ke lobby</button>
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
.connecting button { background: #e3ddcb; color: #333; }
</style>
