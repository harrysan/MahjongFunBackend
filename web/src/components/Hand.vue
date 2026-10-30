<script setup lang="ts">
import { computed, onUnmounted, ref, watch, type ComponentPublicInstance } from 'vue'
import Tile from './Tile.vue'
import { arrangeHand, moveTile } from '../hand'
import type { Tile as T } from '../types'

const props = defineProps<{
  hand: T[]
  drawn?: T
  discardable: Set<number>
  order: number[] | null // the player's arrangement, null = sorted
  confirmTap?: boolean // touch screens: first tap picks a tile, second tap discards it
}>()
const emit = defineEmits<{ discard: [id: number]; reorder: [ids: number[]] }>()

// While dragging, the hand shows this order; it is sent on release.
const preview = ref<number[] | null>(null)
const shown = computed(() => arrangeHand(props.hand, props.drawn, preview.value ?? props.order))
const canDiscard = computed(() => props.discardable.size > 0)

// Drag to rearrange: works with mouse and touch (pointer events).
const DRAG_START_PX = 8
let drag: { id: number; pointerId: number; x: number; y: number; started: boolean } | null = null
let suppressClick = false
const dragging = ref<number | null>(null)

function onPointerDown(e: PointerEvent, id: number) {
  if (e.button !== 0 || drag) return
  drag = { id, pointerId: e.pointerId, x: e.clientX, y: e.clientY, started: false }
  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  window.addEventListener('pointercancel', onPointerUp)
}

function onPointerMove(e: PointerEvent) {
  if (!drag || e.pointerId !== drag.pointerId) return
  if (!drag.started) {
    if (Math.hypot(e.clientX - drag.x, e.clientY - drag.y) < DRAG_START_PX) return
    drag.started = true
    dragging.value = drag.id
    selected.value = null
    preview.value = shown.value.tiles.map((t) => t.id)
  }
  e.preventDefault()
  follow(e)
}

const group = ref<ComponentPublicInstance | null>(null)

/** Moves the dragged tile to the slot nearest the pointer. */
function follow(e: PointerEvent) {
  const target = nearestSlot(e.clientX, e.clientY)
  if (target === null || target === drag!.id) return
  // The drawn tile (not placed yet) counts as "the end of the hand".
  const inHand = preview.value!.includes(target)
  preview.value = moveTile(preview.value!, drag!.id, inHand ? target : null)
}

/**
 * The tile whose slot centre is nearest to the point. It uses layout
 * positions (offsetLeft/Top), which ignore the slide animation's transforms,
 * so the answer does not depend on where tiles are mid-animation.
 */
function nearestSlot(clientX: number, clientY: number): number | null {
  const el = group.value?.$el as HTMLElement | undefined
  if (!el) return null
  const box = el.getBoundingClientRect()
  const x = clientX - box.left
  const y = clientY - box.top
  let best: number | null = null
  let bestDist = Infinity
  for (const s of el.querySelectorAll<HTMLElement>('[data-tile-id]')) {
    const d = Math.hypot(s.offsetLeft + s.offsetWidth / 2 - x, s.offsetTop + s.offsetHeight / 2 - y)
    if (d < bestDist) {
      bestDist = d
      best = Number(s.dataset.tileId)
    }
  }
  return best
}

function onPointerUp(e: PointerEvent) {
  if (!drag || e.pointerId !== drag.pointerId) return
  if (drag.started) {
    follow(e) // the layout has settled since the last move
    suppressClick = true // the click that follows the release is not a discard
    setTimeout(() => (suppressClick = false), 0)
    emit('reorder', preview.value!)
  }
  endDrag()
}

function endDrag() {
  drag = null
  dragging.value = null
  preview.value = null
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('pointercancel', onPointerUp)
}
onUnmounted(endDrag)

// Tap / click to discard.
const selected = ref<number | null>(null)
watch(
  () => props.discardable,
  () => (selected.value = null),
)

function tap(id: number) {
  if (suppressClick || !props.discardable.has(id)) return
  if (props.confirmTap && selected.value !== id) {
    selected.value = id
    return
  }
  selected.value = null
  emit('discard', id)
}
</script>

<template>
  <TransitionGroup ref="group" tag="div" name="hand" class="hand" :class="{ active: canDiscard, dragging: dragging !== null }">
    <span
      v-for="t in shown.tiles"
      :key="t.id"
      class="slot"
      :class="{ lifted: dragging === t.id, picked: selected === t.id }"
      :data-tile-id="t.id"
      @pointerdown="onPointerDown($event, t.id)"
    >
      <Tile :kind="t.kind" :clickable="discardable.has(t.id)" :highlight="t.id === drawn?.id" @click="tap(t.id)" />
    </span>
    <span v-if="shown.drawn" key="gap" class="gap" />
    <span
      v-if="shown.drawn"
      :key="shown.drawn.id"
      class="slot drawn"
      :class="{ lifted: dragging === shown.drawn.id, picked: selected === shown.drawn.id }"
      :data-tile-id="shown.drawn.id"
      @pointerdown="onPointerDown($event, shown.drawn.id)"
    >
      <Tile
        :kind="shown.drawn.kind"
        :clickable="discardable.has(shown.drawn.id)"
        highlight
        @click="tap(shown.drawn!.id)"
      />
    </span>
  </TransitionGroup>
  <p v-if="confirmTap && selected !== null" class="tap-hint">Ketuk lagi untuk membuang</p>
</template>

<style scoped>
.hand {
  position: relative; /* the slots' offsetParent, see nearestSlot */
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  align-items: flex-end;
  gap: 2px;
  padding-top: calc(var(--tile-w) * 0.3);
}
.slot {
  display: inline-flex;
  touch-action: none; /* let a finger drag the tile instead of scrolling */
  cursor: grab;
}
.dragging, .dragging .slot { cursor: grabbing; }
.slot.lifted { transform: translateY(-30%); opacity: 0.85; z-index: 1; }
.slot.lifted :deep(.tile) { box-shadow: 0 10px 18px rgba(0, 0, 0, 0.45); }
.slot.picked { transform: translateY(-22%); }
.slot.picked :deep(.tile) { outline: 3px solid #ff7a59; outline-offset: 1px; }
.gap { width: calc(var(--tile-w) * 0.4); }
.tap-hint { margin: 2px 0 0; text-align: center; font-size: 12px; color: #ffcf3f; }

/* A drawn tile drops in; rearranged tiles slide to their new place. */
.hand-move { transition: transform 0.18s ease; }
.hand-enter-active { transition: transform 0.25s ease-out, opacity 0.25s ease-out; }
.hand-enter-from { transform: translateY(-60%); opacity: 0; }
.hand-leave-active { display: none; }
</style>
