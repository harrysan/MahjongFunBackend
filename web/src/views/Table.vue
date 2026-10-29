<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useGameStore } from '../stores/game'
import { seatWind, tileName } from '../tiles'
import Seat from '../components/Seat.vue'
import Hand from '../components/Hand.vue'
import DiscardPile from '../components/DiscardPile.vue'
import ActionBar from '../components/ActionBar.vue'
import Tile from '../components/Tile.vue'

const game = useGameStore()
const { view, room, log, error, dice, deadline, timeLimit, mySeat, isHost, isSolo, legal, result, discardable } = storeToRefs(game)

// Leaving a running multiplayer round hands your seat to a bot; ask first.
function exit() {
  const running = view.value && !result.value
  if (running && !isSolo.value && !window.confirm('Keluar dari permainan? Kursimu akan dimainkan bot.')) return
  game.leave()
}

// Seats relative to me: play goes clockwise, so the next player sits on my left.
const pos = computed(() => {
  const me = mySeat.value
  return { bottom: me, left: (me + 1) % 4, top: (me + 2) % 4, right: (me + 3) % 4 }
})

function seatProps(s: number) {
  const v = view.value!
  return {
    name: game.seatName(s),
    wind: seatWind(s, v.dealer),
    isDealer: s === v.dealer,
    active: v.phase !== 'Ended' && (v.pending ?? []).includes(s),
    isBot: room.value?.seats[s]?.kind === 'bot',
    melds: v.melds[s] ?? [],
    bonus: v.bonus[s] ?? [],
  }
}

const hint = computed(() => {
  const v = view.value
  if (!v || v.phase === 'Ended') return ''
  if (v.phase === 'ClaimWindow') {
    const t = v.last_discard ? tileName(v.last_discard.kind) : 'tile'
    if (legal.value.length) return `${game.seatName(v.last_discarder)} membuang ${t}. Mau klaim?`
    return `Menunggu klaim atas ${t}…`
  }
  if (v.turn === mySeat.value) {
    return legal.value.some((a) => a.type !== 'discard')
      ? 'Giliranmu: buang satu tile, atau pilih aksi.'
      : 'Giliranmu: klik tile untuk dibuang.'
  }
  return `Giliran ${game.seatName(v.turn)}…`
})

// Countdown of the current turn / claim window.
const now = ref(Date.now())
let ticker: number | undefined
onMounted(() => (ticker = window.setInterval(() => (now.value = Date.now()), 200)))
onUnmounted(() => clearInterval(ticker))

const clock = computed(() => {
  if (deadline.value === null || timeLimit.value <= 0) return null
  const left = Math.max(0, deadline.value - now.value)
  return { seconds: Math.ceil(left / 1000), fraction: left / timeLimit.value, mine: legal.value.length > 0 }
})

const logBox = ref<HTMLElement | null>(null)
watch(
  () => log.value.length,
  async () => {
    await nextTick()
    logBox.value?.scrollTo({ top: logBox.value.scrollHeight })
  },
)
</script>

<template>
  <main v-if="view" class="table">
    <header class="bar">
      <button type="button" class="link" @click="exit">← Keluar</button>
      <span v-if="!isSolo && room" class="room-code">Room <b>{{ room.room }}</b></span>
      <span class="wall">Sisa wall: <b>{{ view.wall_remaining }}</b></span>
    </header>

    <section class="board">
      <Seat class="top" v-bind="seatProps(pos.top)" :hand-count="view.hand_counts[pos.top]" />
      <Seat class="left" v-bind="seatProps(pos.left)" :hand-count="view.hand_counts[pos.left]" />
      <Seat class="right" v-bind="seatProps(pos.right)" :hand-count="view.hand_counts[pos.right]" />

      <div class="center">
        <DiscardPile class="d-top" :tiles="view.discards[pos.top]" :last-id="view.last_discard?.id" />
        <DiscardPile class="d-left" :tiles="view.discards[pos.left]" :last-id="view.last_discard?.id" />
        <div class="info">
          <div class="hint">{{ hint }}</div>
          <div v-if="clock" class="clock" :class="{ urgent: clock.mine && clock.seconds <= 5 }">
            <div class="clock-bar"><span :style="{ width: clock.fraction * 100 + '%' }" /></div>
            <span class="clock-text">{{ clock.seconds }} dtk</span>
          </div>
          <div v-if="view.phase === 'ClaimWindow' && view.last_discard" class="claim-tile">
            <Tile :kind="view.last_discard.kind" highlight />
          </div>
        </div>
        <DiscardPile class="d-right" :tiles="view.discards[pos.right]" :last-id="view.last_discard?.id" />
        <DiscardPile class="d-bottom" :tiles="view.discards[pos.bottom]" :last-id="view.last_discard?.id" />
      </div>

      <Seat class="bottom" v-bind="seatProps(pos.bottom)">
        <ActionBar :actions="legal" :hand="view.hand" @act="game.act" />
        <Hand :hand="view.hand" :drawn="view.drawn" :discardable="discardable" @discard="game.discard" />
      </Seat>
    </section>

    <aside ref="logBox" class="log" aria-label="Riwayat">
      <div v-for="(line, i) in log" :key="i">{{ line }}</div>
    </aside>

    <div v-if="error" class="toast" role="alert">{{ error }}</div>
    <div v-if="dice" class="dice-banner">
      🎲 {{ dice.dice[0] }} + {{ dice.dice[1] }} = {{ dice.dice[0] + dice.dice[1] }} · Dealer baru:
      {{ game.seatName(dice.newDealer) }}
    </div>

    <div v-if="result" class="overlay">
      <div class="dialog">
        <h2>{{ result.draw ? 'Seri' : result.winner === mySeat ? 'Kamu menang!' : 'Ronde selesai' }}</h2>
        <p>{{ game.resultText(result) }}</p>
        <Tile v-if="!result.draw" :kind="result.win_tile.kind" highlight />
        <div class="dialog-actions">
          <button v-if="isHost" type="button" class="primary" @click="game.nextRound()">Ronde berikutnya</button>
          <span v-else>Menunggu host…</span>
          <button type="button" class="secondary" @click="game.leave()">Keluar</button>
        </div>
      </div>
    </div>
  </main>

  <main v-else class="loading">
    <p>Menyiapkan meja…</p>
    <button type="button" class="secondary" @click="game.leave()">Kembali</button>
  </main>

</template>

<style scoped>
.table {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  padding: 8px 16px 16px;
  gap: 8px;
}
.bar { display: flex; justify-content: space-between; align-items: center; color: #e8f1ea; }
.link { background: none; border: none; color: inherit; font: inherit; cursor: pointer; padding: 4px 0; }
.wall b, .room-code b { color: #ffcf3f; }
.room-code { letter-spacing: 1px; }

.board {
  flex: 1;
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 3fr) minmax(0, 1fr);
  grid-template-areas:
    'top top top'
    'left center right'
    'bottom bottom bottom';
  gap: 8px;
  align-items: center;
}
.top { grid-area: top; }
.left { grid-area: left; }
.right { grid-area: right; }
.bottom { grid-area: bottom; }

.center {
  grid-area: center;
  display: grid;
  grid-template-columns: 1fr 1.4fr 1fr;
  grid-template-areas:
    '. dtop .'
    'dleft info dright'
    '. dbottom .';
  gap: 8px;
  padding: 10px;
  border-radius: 16px;
  background: rgba(0, 0, 0, 0.18);
  align-items: center;
}
.d-top { grid-area: dtop; justify-content: center; }
.d-left { grid-area: dleft; justify-content: flex-end; }
.d-right { grid-area: dright; }
.d-bottom { grid-area: dbottom; justify-content: center; }
.info { grid-area: info; display: flex; flex-direction: column; align-items: center; gap: 8px; text-align: center; }
.hint { color: #fff8e0; font-weight: 600; min-height: 2.6em; }
.clock { display: flex; align-items: center; gap: 8px; width: 100%; max-width: 220px; color: #fff8e0; font-size: 13px; }
.clock-bar { flex: 1; height: 6px; border-radius: 3px; background: rgba(0, 0, 0, 0.35); overflow: hidden; }
.clock-bar span { display: block; height: 100%; background: #ffcf3f; transition: width 0.2s linear; }
.clock.urgent { color: #ffb4a8; font-weight: 700; }
.clock.urgent .clock-bar span { background: #e5483d; }
.clock-text { min-width: 4ch; text-align: right; font-variant-numeric: tabular-nums; }

.log {
  max-height: 96px;
  overflow-y: auto;
  padding: 6px 10px;
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.25);
  color: #cfe3d6;
  font-size: 13px;
  line-height: 1.5;
}

.toast, .dice-banner {
  position: fixed;
  left: 50%;
  transform: translateX(-50%);
  padding: 10px 16px;
  border-radius: 10px;
  font-weight: 600;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
  z-index: 20;
  max-width: calc(100vw - 32px);
}
.toast { top: 16px; background: #b3261e; color: #fff; }
.dice-banner { top: 64px; background: #fff8e0; color: #3a2a00; }

.overlay {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  background: rgba(0, 0, 0, 0.5);
  z-index: 10;
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
.dialog h2 { margin: 0; }
.dialog p { margin: 0; }
.dialog-actions { display: flex; flex-wrap: wrap; gap: 10px; justify-content: center; align-items: center; margin-top: 4px; }

.primary, .secondary {
  padding: 10px 18px;
  border: none;
  border-radius: 10px;
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}
.primary { background: #1f8a5b; color: #fff; }
.secondary { background: #e3ddcb; color: #333; }

.loading {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: #f3efe2;
}

@media (max-width: 720px) {
  .board {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    grid-template-areas:
      'top top'
      'left right'
      'center center'
      'bottom bottom';
  }
  .center { grid-template-columns: 1fr 1fr; grid-template-areas: 'dleft dtop' 'dright dbottom' 'info info'; }
  .d-left, .d-top, .d-bottom { justify-content: flex-start; }
}
</style>
