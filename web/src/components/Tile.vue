<script setup lang="ts">
import { computed } from 'vue'
import { tileImage, tileName } from '../tiles'

const props = withDefaults(
  defineProps<{
    kind?: number
    back?: boolean // face down
    size?: 'xs' | 'sm' | 'md'
    clickable?: boolean
    highlight?: boolean
    dim?: boolean
  }>(),
  { size: 'md' },
)
defineEmits<{ click: [] }>()

const faceUp = computed(() => !props.back && props.kind !== undefined)
const src = computed(() => (faceUp.value ? tileImage(props.kind!) : undefined))
const name = computed(() => (faceUp.value ? tileName(props.kind!) : undefined))
</script>

<template>
  <component
    :is="clickable ? 'button' : 'span'"
    :type="clickable ? 'button' : undefined"
    class="tile"
    :class="[size, { back: !faceUp, highlight, dim, clickable }]"
    :title="name"
    @click="clickable && $emit('click')"
  >
    <img v-if="src" :src="src" :alt="name" draggable="false" />
  </component>
</template>

<style scoped>
.tile {
  --w: var(--tile-w);
  display: inline-block;
  width: var(--w);
  height: calc(var(--w) * 1.33);
  padding: 0;
  border: none;
  border-radius: calc(var(--w) * 0.12);
  background: none;
  /* the "thickness" of the tile under the face */
  box-shadow: 0 calc(var(--w) * 0.07) 0 #c9b88c, 0 calc(var(--w) * 0.1) calc(var(--w) * 0.12) rgba(0, 0, 0, 0.35);
  user-select: none;
  flex: none;
  transition: transform 0.12s ease;
}
.xs { --w: calc(var(--tile-w) * 0.55); }
.sm { --w: calc(var(--tile-w) * 0.72); }

img {
  display: block;
  width: 100%;
  height: 100%;
  border-radius: inherit;
  pointer-events: none;
}

.back {
  background: linear-gradient(135deg, #2f7d5a, #1d5a40);
  border: 1px solid #174a34;
  box-shadow: 0 calc(var(--w) * 0.07) 0 #e8dfc6, 0 calc(var(--w) * 0.1) calc(var(--w) * 0.12) rgba(0, 0, 0, 0.35);
}

.clickable { cursor: pointer; }
.clickable:hover, .clickable:focus-visible {
  transform: translateY(-18%);
  outline: none;
}
.highlight { outline: 3px solid #ffcf3f; outline-offset: 1px; }
.dim { filter: brightness(0.8) saturate(0.7); }
</style>
