<script setup lang="ts">
import Tile from './Tile.vue'
import type { Tile as T } from '../types'

defineProps<{
  tiles: T[]
  lastId?: number // the discard that can still be claimed
}>()
</script>

<template>
  <TransitionGroup tag="div" name="toss" class="pile">
    <Tile v-for="t in tiles" :key="t.id" :kind="t.kind" size="xs" :highlight="t.id === lastId" />
  </TransitionGroup>
</template>

<style scoped>
.pile {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  align-content: flex-start;
  min-height: calc(var(--tile-w) * 0.8);
}
/* A discarded tile is tossed onto the table. */
.toss-enter-active { animation: toss 0.3s ease-out; }
/* A claimed tile leaves the pile at once (it shows up in the meld). */
.toss-leave-active { display: none; }
@keyframes toss {
  from { transform: scale(1.8) rotate(-12deg); opacity: 0; }
  60% { transform: scale(0.95) rotate(2deg); opacity: 1; }
  to { transform: none; }
}
</style>
