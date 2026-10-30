<script setup lang="ts">
import Tile from './Tile.vue'
import type { Meld, Tile as T } from '../types'

withDefaults(
  defineProps<{
    name: string
    wind: string
    isDealer: boolean
    active: boolean // the game is waiting on this seat
    isBot: boolean
    away: boolean // the player's connection dropped; a bot plays for now
    melds: Meld[]
    bonus: T[]
    handCount?: number // set for other players: their hand is shown face down
    compact?: boolean // small screens: a count instead of face-down tiles
    callout?: { text: string; key: number } | null // "Pung!", "Hu!" ...
  }>(),
  { compact: false, callout: null },
)
</script>

<template>
  <div class="seat" :class="{ active, compact }">
    <div class="label">
      <span class="wind" :class="{ dealer: isDealer }">{{ wind }}</span>
      <span class="name">{{ name }}</span>
      <span v-if="isBot" class="tag">bot</span>
      <span v-if="away" class="tag away" title="Koneksi pemain terputus; bot bermain untuknya sementara">terputus</span>
      <span v-if="compact && handCount !== undefined" class="count" :title="`${handCount} tile di tangan`">
        <Tile back size="xs" /> ×{{ handCount }}
      </span>
    </div>
    <div v-if="handCount !== undefined && !compact" class="row hidden-hand">
      <Tile v-for="i in handCount" :key="i" back size="xs" />
    </div>
    <TransitionGroup v-if="melds.length || bonus.length" tag="div" name="meld" class="row melds">
      <span v-for="(m, i) in melds" :key="m.tiles[0]?.id ?? i" class="meld">
        <Tile
          v-for="(t, j) in m.tiles"
          :key="t.id"
          :kind="t.kind"
          :back="m.type === 'concealed_kong' && (j === 0 || j === 3)"
          :size="compact && handCount !== undefined ? 'xs' : 'sm'"
        />
      </span>
      <span v-if="bonus.length" key="bonus" class="meld bonus">
        <Tile v-for="t in bonus" :key="t.id" :kind="t.kind" size="xs" />
      </span>
    </TransitionGroup>
    <slot />
    <Transition name="callout">
      <div v-if="callout" :key="callout.key" class="callout" aria-live="polite">{{ callout.text }}</div>
    </Transition>
  </div>
</template>

<style scoped>
.seat {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  min-width: 0;
  padding: 8px;
  border-radius: 12px;
  border: 2px solid transparent;
  transition: border-color 0.2s, background 0.2s;
}
.seat.active {
  border-color: #ffcf3f;
  background: rgba(255, 207, 63, 0.08);
}
.seat.compact { gap: 4px; padding: 4px; }
.label { display: flex; align-items: center; gap: 6px; max-width: 100%; color: #f3efe2; font-weight: 600; }
.name { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.compact .label { gap: 4px; font-size: 13px; flex-wrap: wrap; justify-content: center; }
.wind {
  flex: none;
  display: inline-block;
  padding: 1px 8px;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.3);
  font-size: 13px;
}
.compact .wind { padding: 1px 6px; font-size: 11px; }
.wind.dealer { background: #c62828; }
.tag { flex: none; font-size: 11px; padding: 1px 6px; border-radius: 4px; background: rgba(255, 255, 255, 0.15); font-weight: 400; }
.tag.away { background: #b3261e; }
.count { flex: none; display: inline-flex; align-items: center; gap: 3px; font-size: 12px; font-weight: 600; color: #cfe3d6; }
.count :deep(.tile) { --w: calc(var(--tile-w) * 0.4); }
.row { display: flex; flex-wrap: wrap; justify-content: center; gap: 8px; }
.compact .row { gap: 4px; }
.hidden-hand { gap: 1px; }
.meld { display: inline-flex; gap: 1px; }
.bonus { opacity: 0.95; }

/* A new meld pops in. */
.meld-enter-active { animation: pop 0.35s ease-out; }
@keyframes pop {
  from { transform: scale(0.4); opacity: 0; }
  70% { transform: scale(1.12); opacity: 1; }
  to { transform: none; }
}

.callout {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  padding: 4px 18px;
  border-radius: 999px;
  background: #d4352b;
  color: #fff;
  font-size: clamp(22px, 4vw, 34px);
  font-weight: 900;
  letter-spacing: 1px;
  white-space: nowrap;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.4);
  pointer-events: none;
  z-index: 5;
}
.callout-enter-active { animation: callout-in 0.4s cubic-bezier(0.2, 1.6, 0.4, 1); }
.callout-leave-active { transition: opacity 0.3s; }
.callout-leave-to { opacity: 0; }
@keyframes callout-in {
  from { transform: translate(-50%, -50%) scale(0.2); opacity: 0; }
  to { transform: translate(-50%, -50%) scale(1); opacity: 1; }
}
</style>
