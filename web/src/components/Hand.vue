<script setup lang="ts">
import { computed } from 'vue'
import Tile from './Tile.vue'
import type { Tile as T } from '../types'

const props = defineProps<{
  hand: T[]
  drawn?: T
  discardable: Set<number>
}>()
defineEmits<{ discard: [id: number] }>()

// The just-drawn tile sits apart on the right, like at a real table.
const rest = computed(() => props.hand.filter((t) => t.id !== props.drawn?.id))
const canDiscard = computed(() => props.discardable.size > 0)
</script>

<template>
  <div class="hand" :class="{ active: canDiscard }">
    <Tile
      v-for="t in rest"
      :key="t.id"
      :kind="t.kind"
      :clickable="discardable.has(t.id)"
      @click="$emit('discard', t.id)"
    />
    <span v-if="drawn" class="gap" />
    <Tile
      v-if="drawn"
      :key="drawn.id"
      :kind="drawn.kind"
      :clickable="discardable.has(drawn.id)"
      highlight
      @click="$emit('discard', drawn.id)"
    />
  </div>
</template>

<style scoped>
.hand {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  align-items: flex-end;
  gap: 2px;
  padding-top: calc(var(--tile-w) * 0.3);
}
.gap { width: calc(var(--tile-w) * 0.4); }
</style>
