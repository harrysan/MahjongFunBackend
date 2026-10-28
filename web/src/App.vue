<script setup lang="ts">
import { useGameStore } from './stores/game'
import { useConnectionStore } from './stores/connection'
import Lobby from './views/Lobby.vue'
import Table from './views/Table.vue'

const game = useGameStore()
const conn = useConnectionStore()
</script>

<template>
  <Table v-if="conn.status !== 'idle' || game.view" />
  <Lobby v-else />
</template>
