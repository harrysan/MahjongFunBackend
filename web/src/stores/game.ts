import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { useConnectionStore } from './connection'
import { tileName } from '../tiles'
import type { Action, GameEvent, GameResult, PlayerView, RoomState, Seat, ServerMessage, Settings } from '../types'

const MELD_VERBS: Record<string, string> = {
  chi: 'chi',
  pung: 'pung',
  kong: 'kong',
  concealed_kong: 'kong tertutup',
}
const MAX_LOG = 40
const CODE_CHARS = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789' // no 0/O, 1/I
const SOLO_PREFIX = 'SOLO-'

function randomCode(n: number): string {
  let s = ''
  for (let i = 0; i < n; i++) s += CODE_CHARS[Math.floor(Math.random() * CODE_CHARS.length)]
  return s
}

/** Show the room code in the address bar so it can be shared as an invite link. */
function setURLRoom(code: string | null) {
  const url = new URL(location.href)
  if (code) url.searchParams.set('room', code)
  else url.searchParams.delete('room')
  history.replaceState(null, '', url)
}

/** A join_room request that the server has not confirmed yet. */
interface PendingJoin {
  name: string
  code: string
  create: boolean
  solo: boolean
  settings?: Settings
}

/**
 * Everything the table shows. It is filled only from server messages;
 * the client never applies game rules itself.
 */
export const useGameStore = defineStore('game', () => {
  const conn = useConnectionStore()

  const room = ref<RoomState | null>(null)
  const view = ref<PlayerView | null>(null)
  const log = ref<string[]>([])
  const error = ref<string | null>(null)
  const dice = ref<{ dice: [number, number]; newDealer: Seat } | null>(null)
  // Clock of the current turn / claim window, as local timestamps (ms).
  const deadline = ref<number | null>(null)
  const timeLimit = ref(0)
  // Why the last attempt to enter a room failed; shown in the lobby.
  const lobbyError = ref<string | null>(null)
  let pending: PendingJoin | null = null
  let errorTimer: number | undefined
  let diceTimer: number | undefined

  const mySeat = computed(() => room.value?.your_seat ?? 0)
  const isHost = computed(() => room.value !== null && room.value.host === room.value.your_seat)
  const isSolo = computed(() => room.value?.room.startsWith(SOLO_PREFIX) ?? false)
  const legal = computed<Action[]>(() => view.value?.legal_actions ?? [])
  const result = computed<GameResult | undefined>(() => view.value?.result)
  const discardable = computed(() => new Set(legal.value.filter((a) => a.type === 'discard').map((a) => a.tile_id)))

  function seatName(s: Seat): string {
    if (s === mySeat.value) return 'Kamu'
    return room.value?.seats[s]?.name || `Kursi ${s + 1}`
  }

  function addLog(line: string) {
    log.value.push(line)
    if (log.value.length > MAX_LOG) log.value.splice(0, log.value.length - MAX_LOG)
  }

  function showError(msg: string) {
    error.value = msg
    clearTimeout(errorTimer)
    errorTimer = window.setTimeout(() => (error.value = null), 3000)
  }

  function resultText(r: GameResult): string {
    if (r.draw) return 'Wall habis. Ronde seri.'
    if (r.self_draw) return `${seatName(r.winner)} hu dari wall (${tileName(r.win_tile.kind)})!`
    return `${seatName(r.winner)} hu dari buangan ${seatName(r.from)} (${tileName(r.win_tile.kind)})!`
  }

  function onEvent(e: GameEvent) {
    const who = seatName(e.seat)
    const tiles = (e.tiles ?? []).map((t) => tileName(t.kind)).join(', ')
    switch (e.type) {
      case 'discarded':
        addLog(`${who} membuang ${tiles}`)
        break
      case 'claimed':
        addLog(`${who} ${MELD_VERBS[e.meld ?? ''] ?? 'klaim'}${tiles ? ': ' + tiles : ''}`)
        break
      case 'bonus':
        addLog(`${who} mendapat ${tiles}`)
        break
      case 'game_end':
        if (e.result) addLog(resultText(e.result))
        break
      case 'dice_rolled':
        if (e.dice && e.new_dealer !== undefined) {
          dice.value = { dice: e.dice, newDealer: e.new_dealer }
          clearTimeout(diceTimer)
          diceTimer = window.setTimeout(() => (dice.value = null), 4000)
          const [a, b] = e.dice
          addLog(`${who} melempar dadu: ${a} + ${b} = ${a + b}. Dealer baru: ${seatName(e.new_dealer)}`)
        }
        break
    }
  }

  function onMessage(m: ServerMessage) {
    switch (m.type) {
      case 'room_state':
        if (!room.value) onJoined(m.data)
        room.value = m.data
        break
      case 'game_state':
        if (view.value?.phase === 'Ended' && m.data.phase !== 'Ended') {
          addLog('— Ronde baru —')
        }
        view.value = m.data
        if (m.data.time_left_ms > 0 && m.data.phase !== 'Ended') {
          deadline.value = Date.now() + m.data.time_left_ms
          timeLimit.value = m.data.time_limit_ms
        } else {
          deadline.value = null
        }
        break
      case 'event':
        onEvent(m.data)
        break
      case 'error':
        if (!room.value && pending) onJoinFailed(m.data.code, m.data.message)
        else showError(m.data.message)
        break
      case 'claim_window':
        // The same options are in game_state.legal_actions; nothing extra to store yet.
        break
    }
  }

  function sendJoin(p: PendingJoin) {
    pending = p
    conn.send('join_room', { room: p.code, name: p.name, create: p.create })
  }

  function onJoined(st: RoomState) {
    const p = pending
    pending = null
    if (!p?.solo) setURLRoom(st.room)
    if (!p?.create) return
    if (p.settings) conn.send('update_settings', p.settings)
    if (p.solo) {
      for (let s = 1; s < 4; s++) conn.send('add_bot', { seat: s })
      conn.send('start_game')
    }
  }

  function onJoinFailed(code: string, message: string) {
    if (code === 'room_exists' && pending?.create) {
      // Random code collided with an open room; try another one.
      sendJoin({ ...pending, code: (pending.solo ? SOLO_PREFIX : '') + randomCode(pending.solo ? 8 : 5) })
      return
    }
    pending = null
    conn.disconnect()
    lobbyError.value = message
  }

  // The server could not be reached, or dropped us before we got in.
  watch(
    () => conn.status,
    (st) => {
      if (st === 'closed' && !room.value) {
        pending = null
        conn.disconnect()
        lobbyError.value = 'Tidak bisa terhubung ke server.'
      }
    },
  )

  function enter(p: PendingJoin) {
    reset()
    lobbyError.value = null
    conn.connect(onMessage)
    sendJoin(p)
  }

  /** Solo mode: a private room with three bots, started right away (SPEC 1). */
  function startSolo(name: string, settings: Settings) {
    enter({ name, code: SOLO_PREFIX + randomCode(8), create: true, solo: true, settings })
  }

  /** Make a new room for friends; the caller becomes host. */
  function createRoom(name: string, settings: Settings) {
    enter({ name, code: randomCode(5), create: true, solo: false, settings })
  }

  function joinRoom(name: string, code: string) {
    enter({ name, code: code.trim().toUpperCase(), create: false, solo: false })
  }

  function leave() {
    pending = null
    conn.disconnect()
    reset()
    setURLRoom(null)
  }

  function reset() {
    room.value = null
    view.value = null
    log.value = []
    dice.value = null
    error.value = null
    deadline.value = null
  }

  // Waiting room, host only (the server checks).
  function addBot(seat: Seat) {
    conn.send('add_bot', { seat })
  }
  function removeBot(seat: Seat) {
    conn.send('remove_bot', { seat })
  }
  function fillWithBots() {
    room.value?.seats.forEach((s) => s.kind === 'empty' && addBot(s.seat))
  }
  function updateSettings(settings: Settings) {
    conn.send('update_settings', settings)
  }
  function startGame() {
    conn.send('start_game')
  }

  function discard(tileId: number) {
    if (discardable.value.has(tileId)) conn.send('discard', { tile_id: tileId })
  }

  function act(a: Action) {
    if (a.type === 'discard') return discard(a.tile_id)
    if (view.value?.phase === 'ClaimWindow') {
      const data: Record<string, unknown> = { action: a.type }
      if (a.type === 'chi') data.tiles = [a.tile_id, a.tile_id2]
      conn.send('claim', data)
    } else {
      const data: Record<string, unknown> = { action: a.type }
      if (a.type === 'kong') data.tile_id = a.tile_id
      conn.send('self_action', data)
    }
  }

  function nextRound() {
    conn.send('next_round')
  }

  return {
    room, view, log, error, dice, deadline, timeLimit, lobbyError,
    mySeat, isHost, isSolo, legal, result, discardable,
    seatName, resultText,
    startSolo, createRoom, joinRoom, leave,
    addBot, removeBot, fillWithBots, updateSettings, startGame,
    discard, act, nextRound,
  }
})
