<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useGameStore } from '../stores/game'
import { seatWind, tileName } from '../tiles'
import { muted, playSound, setMuted } from '../sound'
import { COMPACT_QUERY, TOUCH_QUERY, useMedia } from '../media'
import Seat from '../components/Seat.vue'
import Hand from '../components/Hand.vue'
import DiscardPile from '../components/DiscardPile.vue'
import ActionBar from '../components/ActionBar.vue'
import DiceRoll from '../components/DiceRoll.vue'
import Tile from '../components/Tile.vue'

const game = useGameStore()
const { view, room, log, error, dice, callout, handOrder, deadline, timeLimit, mySeat, isHost, isSolo, legal, result, discardable } =
  storeToRefs(game)

const compact = useMedia(COMPACT_QUERY)
const touch = useMedia(TOUCH_QUERY)
const portrait = useMedia('(max-width: 720px) and (orientation: portrait)')

// Phones held upright get small tiles; suggest turning the phone, once.
const ROTATE_KEY = 'mahjong.rotateHintSeen'
const rotateHintSeen = ref(readFlag(ROTATE_KEY))
function readFlag(key: string): boolean {
  try {
    return localStorage.getItem(key) === '1'
  } catch {
    return false
  }
}
function dismissRotateHint() {
  rotateHintSeen.value = true
  try {
    localStorage.setItem(ROTATE_KEY, '1')
  } catch {
    // shown again next time; fine
  }
}

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
    away: room.value?.seats[s]?.kind === 'human' && !room.value.seats[s].connected,
    melds: v.melds[s] ?? [],
    bonus: v.bonus[s] ?? [],
    compact: compact.value,
    callout: callout.value?.seat === s ? callout.value : null,
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
    const how = touch.value ? 'ketuk' : 'klik'
    return legal.value.some((a) => a.type !== 'discard')
      ? 'Giliranmu: buang satu tile, atau pilih aksi.'
      : `Giliranmu: ${how} tile untuk dibuang.`
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

// Tell the player the game is waiting for them: a sound, and the tab title
// for players looking at another tab.
const BASE_TITLE = document.title
const needsMe = computed(() => legal.value.length > 0 && view.value?.phase !== 'Ended')
watch(
  needsMe,
  (yes, before) => {
    document.title = yes ? '(!) Giliranmu – ' + BASE_TITLE : BASE_TITLE
    if (yes && before === false) playSound('turn')
  },
  { immediate: true },
)
onUnmounted(() => (document.title = BASE_TITLE))

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
  <main v-if="view" class="table" :class="{ compact, portrait }">
    <header class="bar">
      <button type="button" class="link" @click="exit">← Keluar</button>
      <span v-if="!isSolo && room" class="room-code">Room <b>{{ room.room }}</b></span>
      <span class="wall">Sisa wall: <b>{{ view.wall_remaining }}</b></span>
      <button
        type="button"
        class="icon"
        :aria-label="muted ? 'Nyalakan suara' : 'Matikan suara'"
        :title="muted ? 'Nyalakan suara' : 'Matikan suara'"
        @click="setMuted(!muted)"
      >
        {{ muted ? '🔇' : '🔊' }}
      </button>
    </header>

    <p v-if="portrait && !rotateHintSeen" class="rotate-hint">
      <span>Putar HP ke posisi mendatar agar tile lebih besar.</span>
      <button type="button" class="link" aria-label="Tutup" @click="dismissRotateHint">✕</button>
    </p>

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
            <Tile :kind="view.last_discard.kind" highlight :size="compact ? 'sm' : 'md'" />
          </div>
        </div>
        <DiscardPile class="d-right" :tiles="view.discards[pos.right]" :last-id="view.last_discard?.id" />
        <DiscardPile class="d-bottom" :tiles="view.discards[pos.bottom]" :last-id="view.last_discard?.id" />
      </div>

      <Seat class="bottom mine" v-bind="seatProps(pos.bottom)">
        <ActionBar class="actions" :actions="legal" :hand="view.hand" @act="game.act" />
        <div class="hand-row">
          <Hand
            :hand="view.hand"
            :drawn="view.drawn"
            :discardable="discardable"
            :order="handOrder"
            :confirm-tap="touch"
            @discard="game.discard"
            @reorder="game.arrangeHand"
          />
          <button v-if="handOrder" type="button" class="sort" title="Urutkan tangan otomatis" @click="game.sortHand()">
            Urutkan
          </button>
        </div>
      </Seat>
    </section>

    <aside ref="logBox" class="log" aria-label="Riwayat">
      <div v-for="(line, i) in log" :key="i">{{ line }}</div>
    </aside>

    <div v-if="error" class="toast" role="alert">{{ error }}</div>
    <div v-if="dice" class="dice-layer">
      <DiceRoll
        :key="dice.dice.join() + dice.newDealer + log.length"
        :dice="dice.dice"
        :caption="`Dealer baru: ${game.seatName(dice.newDealer)}`"
      />
    </div>

    <Transition name="dialog">
      <div v-if="result" class="overlay">
        <div class="dialog" :class="{ won: !result.draw && result.winner === mySeat }">
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
    </Transition>
  </main>

  <main v-else class="loading">
    <p>Menyiapkan meja…</p>
    <button type="button" class="secondary" @click="game.leave()">Kembali</button>
  </main>
</template>

<style scoped>
.table {
  min-height: 100vh;
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
  padding: 8px 16px 16px;
  padding-left: max(16px, env(safe-area-inset-left));
  padding-right: max(16px, env(safe-area-inset-right));
  gap: 8px;
}
.bar { display: flex; justify-content: space-between; align-items: center; gap: 12px; color: #e8f1ea; }
.bar .wall { margin-left: auto; }
.link { background: none; border: none; color: inherit; font: inherit; cursor: pointer; padding: 4px 0; }
.icon {
  flex: none;
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.25);
  font-size: 18px;
  cursor: pointer;
}
.wall b, .room-code b { color: #ffcf3f; }
.room-code { letter-spacing: 1px; }
.rotate-hint {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  padding: 6px 10px;
  border-radius: 8px;
  background: rgba(255, 207, 63, 0.15);
  color: #fff8e0;
  font-size: 13px;
}
.rotate-hint span { flex: 1; }

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
.claim-tile { animation: bob 1s ease-in-out infinite; }
@keyframes bob {
  50% { transform: translateY(-4px); }
}

.hand-row { display: flex; align-items: flex-end; justify-content: center; gap: 8px; max-width: 100%; }
.sort {
  flex: none;
  margin-bottom: 4px;
  padding: 4px 10px;
  border: none;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.3);
  color: #fff8e0;
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}

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

.toast {
  position: fixed;
  top: 16px;
  left: 50%;
  transform: translateX(-50%);
  padding: 10px 16px;
  border-radius: 10px;
  font-weight: 600;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
  z-index: 20;
  max-width: calc(100vw - 32px);
  background: #b3261e;
  color: #fff;
}
.dice-layer {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
  z-index: 15;
}

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
  max-height: 100%;
  overflow-y: auto;
  padding: 24px;
  border-radius: 16px;
  background: #fffdf6;
  color: #222;
  text-align: center;
}
.dialog.won { box-shadow: 0 0 0 4px #ffcf3f, 0 0 40px rgba(255, 207, 63, 0.6); }
.dialog h2 { margin: 0; }
.dialog p { margin: 0; }
.dialog-actions { display: flex; flex-wrap: wrap; gap: 10px; justify-content: center; align-items: center; margin-top: 4px; }
.dialog-enter-active { transition: opacity 0.25s; }
.dialog-enter-active .dialog { animation: dialog-in 0.35s cubic-bezier(0.2, 1.4, 0.4, 1); }
.dialog-enter-from { opacity: 0; }
@keyframes dialog-in {
  from { transform: scale(0.7); opacity: 0; }
  to { transform: none; opacity: 1; }
}

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

/* ---- Phones (see COMPACT_QUERY): the whole table fits on one screen. ---- */
.table.compact {
  height: 100dvh;
  min-height: 0;
  padding-top: max(4px, env(safe-area-inset-top));
  padding-bottom: max(4px, env(safe-area-inset-bottom));
  gap: 4px;
  overflow: hidden;
}
.compact .bar { font-size: 13px; }
.compact .icon { width: 30px; height: 30px; font-size: 15px; }
.compact .board { min-height: 0; gap: 4px; align-items: stretch; }
.compact .center { min-height: 0; gap: 4px; padding: 6px; overflow: hidden; }
.compact .hint { min-height: 0; font-size: 13px; }
.compact .info { gap: 4px; }
.compact .seat:not(.mine) { align-self: start; }
/* The action buttons float over the table, so the hand keeps its room. */
.compact .mine { position: relative; padding-top: 0; }
.compact .mine :deep(.label) { display: none; }
.compact .actions { position: absolute; bottom: 100%; left: 0; right: 0; margin-bottom: 4px; z-index: 6; }
.compact .log { max-height: 1.6em; padding: 2px 10px; font-size: 12px; }

/* Landscape phone: opponents left and right, the top one above the discards. */
.compact:not(.portrait) .board {
  grid-template-columns: minmax(0, 1fr) minmax(0, 2.6fr) minmax(0, 1fr);
  grid-template-rows: auto minmax(0, 1fr) auto;
  grid-template-areas:
    'left top right'
    'left center right'
    'bottom bottom bottom';
}
.compact:not(.portrait) .log { display: none; }
.compact:not(.portrait) .claim-tile { display: none; } /* it is highlighted in the pile */
/* Own melds and flowers sit beside the hand instead of above it. */
.compact:not(.portrait) .mine { flex-direction: row; justify-content: center; align-items: flex-end; }
.compact:not(.portrait) .mine :deep(.melds) { flex-wrap: nowrap; padding-bottom: 4px; }

/* Portrait phone: the three opponents in a row. */
.compact.portrait .board {
  grid-template-columns: repeat(3, minmax(0, 1fr));
  grid-template-rows: auto minmax(0, 1fr) auto;
  grid-template-areas:
    'left top right'
    'center center center'
    'bottom bottom bottom';
}
.compact.portrait .center {
  grid-template-columns: 1fr 1fr;
  grid-template-areas: 'dleft dtop' 'dright dbottom' 'info info';
  align-content: start;
}
.compact.portrait .d-left, .compact.portrait .d-top, .compact.portrait .d-bottom { justify-content: flex-start; }
.compact .center { align-content: center; }
</style>
