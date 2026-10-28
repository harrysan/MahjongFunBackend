<script setup lang="ts">
import Tile from './Tile.vue'
import type { Tile as T } from '../types'

defineProps<{
  tiles: T[]
  lastId?: number // the discard that can still be claimed
}>()
</script>

<template>
  <div class="pile">
    <Tile v-for="t in tiles" :key="t.id" :kind="t.kind" size="xs" :highlight="t.id === lastId" />
  </div>
</template>

<style scoped>
.pile {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  align-content: flex-start;
  min-height: calc(var(--tile-w) * 0.8);
}
</style>
