<script setup lang="ts">
import { computed } from 'vue'
import Tile from './Tile.vue'
import type { Action, Tile as T } from '../types'

const props = defineProps<{
  actions: Action[]
  hand: T[]
}>()
defineEmits<{ act: [a: Action] }>()

const LABELS: Record<string, string> = { hu: 'Hu!', kong: 'Kong', pung: 'Pung', chi: 'Chi', pass: 'Pass' }

// Discards are made by clicking a tile in the hand, not here.
const buttons = computed(() => props.actions.filter((a) => a.type !== 'discard'))

function kindOf(id: number): number | undefined {
  return props.hand.find((t) => t.id === id)?.kind
}
</script>

<template>
  <div v-if="buttons.length" class="actions">
    <button
      v-for="(a, i) in buttons"
      :key="i"
      type="button"
      class="btn"
      :class="a.type"
      @click="$emit('act', a)"
    >
      {{ LABELS[a.type] }}
      <span v-if="a.type === 'chi'" class="tiles">
        <Tile :kind="kindOf(a.tile_id)" size="xs" />
        <Tile :kind="kindOf(a.tile_id2)" size="xs" />
      </span>
      <span v-else-if="a.type === 'kong' && kindOf(a.tile_id) !== undefined" class="tiles">
        <Tile :kind="kindOf(a.tile_id)" size="xs" />
      </span>
    </button>
  </div>
</template>

<style scoped>
.actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
}
.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 40px;
  padding: 6px 16px;
  border: none;
  border-radius: 999px;
  font: inherit;
  font-weight: 700;
  font-size: 16px;
  color: #fff;
  background: #3b6fd8;
  cursor: pointer;
  box-shadow: 0 3px 0 rgba(0, 0, 0, 0.3);
}
.btn:active { transform: translateY(2px); box-shadow: 0 1px 0 rgba(0, 0, 0, 0.3); }
.btn.hu { background: #d4352b; font-size: 18px; }
.btn.kong { background: #7a3fc4; }
.btn.pung { background: #e08a12; }
.btn.chi { background: #1f8a5b; }
.btn.pass { background: #5d6670; }
.tiles { display: inline-flex; gap: 2px; }
</style>
