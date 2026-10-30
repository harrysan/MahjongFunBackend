<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'

const props = defineProps<{
  dice: [number, number] // the server's roll
  caption: string // shown once the dice stop
}>()

const ROLL_MS = 1000
const STEP_MS = 75

// Which of the 3x3 cells hold a pip, for faces 1-6.
const PIPS: Record<number, number[]> = {
  1: [4],
  2: [2, 6],
  3: [2, 4, 6],
  4: [0, 2, 6, 8],
  5: [0, 2, 4, 6, 8],
  6: [0, 2, 3, 5, 6, 8],
}

const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
const faces = ref<[number, number]>(reduced ? props.dice : [1, 1])
const rolling = ref(!reduced)
let timer: number | undefined

onMounted(() => {
  if (!rolling.value) return
  const start = Date.now()
  timer = window.setInterval(() => {
    if (Date.now() - start >= ROLL_MS) {
      clearInterval(timer)
      faces.value = props.dice
      rolling.value = false
      return
    }
    faces.value = [1 + Math.floor(Math.random() * 6), 1 + Math.floor(Math.random() * 6)]
  }, STEP_MS)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="dice-roll" role="status">
    <div class="dice">
      <span v-for="(f, i) in faces" :key="i" class="die" :class="{ rolling }" :aria-label="`Dadu ${f}`">
        <span v-for="c in 9" :key="c" class="cell"><i v-if="PIPS[f].includes(c - 1)" /></span>
      </span>
    </div>
    <p class="caption" :class="{ hidden: rolling }">
      {{ dice[0] }} + {{ dice[1] }} = <b>{{ dice[0] + dice[1] }}</b> · {{ caption }}
    </p>
  </div>
</template>

<style scoped>
.dice-roll {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 16px 22px;
  border-radius: 16px;
  background: rgba(255, 248, 224, 0.96);
  color: #3a2a00;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.45);
  pointer-events: none;
}
.dice { display: flex; gap: 14px; }
.die {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  grid-template-rows: repeat(3, 1fr);
  width: 52px;
  height: 52px;
  padding: 6px;
  border-radius: 10px;
  background: #fff;
  box-shadow: inset 0 -3px 0 #d8cfb8, 0 3px 6px rgba(0, 0, 0, 0.3);
}
.die.rolling { animation: tumble 0.3s linear infinite; }
.die:nth-child(2).rolling { animation-duration: 0.36s; animation-direction: reverse; }
.cell { display: flex; align-items: center; justify-content: center; }
.cell i { width: 9px; height: 9px; border-radius: 50%; background: #2a2a2a; }
.die:first-child .cell i { background: #c62828; }
.caption { margin: 0; font-weight: 600; transition: opacity 0.2s; }
.caption.hidden { opacity: 0; }
@keyframes tumble {
  0% { transform: rotate(0) translateY(0); }
  25% { transform: rotate(90deg) translateY(-6px); }
  50% { transform: rotate(180deg) translateY(0); }
  75% { transform: rotate(270deg) translateY(-4px); }
  100% { transform: rotate(360deg) translateY(0); }
}
</style>
