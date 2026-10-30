<script setup lang="ts">
import Tile from './Tile.vue'
import type { Meld, Tile as T } from '../types'

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
}>()
</script>

<template>
  <div class="seat" :class="{ active }">
    <div class="label">
      <span class="wind" :class="{ dealer: isDealer }">{{ wind }}</span>
      <span class="name">{{ name }}</span>
      <span v-if="isBot" class="tag">bot</span>
      <span v-if="away" class="tag away" title="Koneksi pemain terputus; bot bermain untuknya sementara">terputus</span>
    </div>
    <div v-if="handCount !== undefined" class="row hidden-hand">
      <Tile v-for="i in handCount" :key="i" back size="xs" />
    </div>
    <div v-if="melds.length || bonus.length" class="row">
      <span v-for="(m, i) in melds" :key="i" class="meld">
        <Tile
          v-for="(t, j) in m.tiles"
          :key="t.id"
          :kind="t.kind"
          :back="m.type === 'concealed_kong' && (j === 0 || j === 3)"
          size="sm"
        />
      </span>
      <span v-if="bonus.length" class="meld bonus">
        <Tile v-for="t in bonus" :key="t.id" :kind="t.kind" size="xs" />
      </span>
    </div>
    <slot />
  </div>
</template>

<style scoped>
.seat {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 8px;
  border-radius: 12px;
  border: 2px solid transparent;
  transition: border-color 0.2s, background 0.2s;
}
.seat.active {
  border-color: #ffcf3f;
  background: rgba(255, 207, 63, 0.08);
}
.label { display: flex; align-items: center; gap: 6px; color: #f3efe2; font-weight: 600; }
.wind {
  display: inline-block;
  padding: 1px 8px;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.3);
  font-size: 13px;
}
.wind.dealer { background: #c62828; }
.tag.away { background: #b3261e; }
.tag { font-size: 11px; padding: 1px 6px; border-radius: 4px; background: rgba(255, 255, 255, 0.15); font-weight: 400; }
.row { display: flex; flex-wrap: wrap; justify-content: center; gap: 8px; }
.hidden-hand { gap: 1px; }
.meld { display: inline-flex; gap: 1px; }
.bonus { opacity: 0.95; }
</style>
