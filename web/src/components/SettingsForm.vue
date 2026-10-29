<script setup lang="ts">
import { BOT_SPEEDS, CLAIM_OPTIONS, TURN_OPTIONS, secondsLabel, type Prefs } from '../settings'

const props = defineProps<{ modelValue: Prefs; idPrefix?: string }>()
const emit = defineEmits<{ 'update:modelValue': [p: Prefs] }>()

const id = (name: string) => `${props.idPrefix ?? 'set'}-${name}`

function set<K extends keyof Prefs>(key: K, value: Prefs[K]) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}
</script>

<template>
  <div class="settings">
    <label :for="id('bot')">Waktu bot berpikir</label>
    <select :id="id('bot')" :value="modelValue.botSpeed" @change="set('botSpeed', Number(($event.target as HTMLSelectElement).value))">
      <option v-for="(s, i) in BOT_SPEEDS" :key="i" :value="i">{{ s.label }}</option>
    </select>

    <label :for="id('turn')">Batas waktu giliran</label>
    <select :id="id('turn')" :value="modelValue.turn" @change="set('turn', Number(($event.target as HTMLSelectElement).value))">
      <option v-for="n in TURN_OPTIONS" :key="n" :value="n">{{ secondsLabel(n) }}</option>
    </select>
    <small>Jika habis, tile yang baru diambil dibuang otomatis.</small>

    <label :for="id('claim')">Batas waktu klaim (chi/pung/kong/hu)</label>
    <select :id="id('claim')" :value="modelValue.claim" @change="set('claim', Number(($event.target as HTMLSelectElement).value))">
      <option v-for="n in CLAIM_OPTIONS" :key="n" :value="n">{{ secondsLabel(n) }}</option>
    </select>
    <small>Jika habis, dianggap pass.</small>
  </div>
</template>

<style scoped>
.settings { display: flex; flex-direction: column; gap: 6px; }
label { margin-top: 6px; color: #f3efe2; font-weight: 600; font-size: 14px; }
label:first-child { margin-top: 0; }
small { color: #9fbcab; font-size: 12px; }
select {
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid #2c6b4f;
  background: #fffdf6;
  color: #222;
  font: inherit;
  font-size: 16px;
}
</style>
