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
// The server keeps a room with nobody connected for 5 minutes (IdleRoomTimeout).
const RECONNECT_GIVE_UP_MS = 5 * 60 * 1000
const RECONNECT_MAX_DELAY_MS = 10_000
// Per tab, so two tabs can be two different players; it survives a refresh.
const SESSION_KEY = 'mahjong.session'

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

/** What lets this tab get its seat back after a refresh or a dropped connection. */
interface Session {
  room: string
  token: string
}

function loadSession(): Session | null {
  try {
    const s = JSON.parse(sessionStorage.getItem(SESSION_KEY) ?? 'null')
    return s && typeof s.room === 'string' && typeof s.token === 'string' ? s : null
  } catch {
    return null
  }
}

function saveSession(s: Session | null) {
  try {
    if (s) sessionStorage.setItem(SESSION_KEY, JSON.stringify(s))
    else sessionStorage.removeItem(SESSION_KEY)
  } catch {
    // storage unavailable: a refresh will just not reconnect
  }
}

/** A request to enter a room that the server has not confirmed yet. */
type PendingJoin =
  | { kind: 'create' | 'join'; name: string; code: string; solo: boolean; settings?: Settings }
  | { kind: 'rejoin'; code: string; token: string }

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
  // The connection dropped and we are trying to get our seat back.
  const reconnecting = ref(false)
  // The same player opened this game in another tab or device.
  const replaced = ref(false)
  let pending: PendingJoin | null = null
  let errorTimer: number | undefined
  let diceTimer: number | undefined
  let retryTimer: number | undefined
  let retryCount = 0
  let giveUpAt = 0

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
        if (pending) onJoined(m.data)
        room.value = m.data
        if (m.data.token) saveSession({ room: m.data.room, token: m.data.token })
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
        if (pending) onJoinFailed(m.data.code, m.data.message)
        else if (m.data.code === 'session_moved') replaced.value = true
        else showError(m.data.message)
        break
      case 'claim_window':
        // The same options are in game_state.legal_actions; nothing extra to store yet.
        break
    }
  }

  function sendJoin(p: PendingJoin) {
    pending = p
    if (p.kind === 'rejoin') conn.send('rejoin_room', { room: p.code, token: p.token })
    else conn.send('join_room', { room: p.code, name: p.name, create: p.kind === 'create' })
  }

  function onJoined(st: RoomState) {
    const p = pending!
    pending = null
    if (reconnecting.value) addLog('— Tersambung kembali —')
    stopReconnect()
    if (!st.room.startsWith(SOLO_PREFIX)) setURLRoom(st.room)
    if (p.kind !== 'create') return
    if (p.settings) conn.send('update_settings', p.settings)
    if (p.solo) {
      for (let s = 1; s < 4; s++) conn.send('add_bot', { seat: s })
      conn.send('start_game')
    }
  }

  function onJoinFailed(code: string, message: string) {
    const p = pending!
    if (code === 'room_exists' && p.kind === 'create') {
      // Random code collided with an open room; try another one.
      sendJoin({ ...p, code: (p.solo ? SOLO_PREFIX : '') + randomCode(p.solo ? 8 : 5) })
      return
    }
    if (p.kind === 'rejoin') {
      // The room closed or our seat is gone: the old game is over.
      message = 'Permainan sebelumnya sudah berakhir.'
    }
    pending = null
    goToLobby(message)
  }

  // Connection lost. In a room: keep trying to get the seat back.
  // Before getting in: the server could not be reached.
  watch(
    () => conn.status,
    (st) => {
      if (st !== 'closed') return
      if (room.value && !replaced.value) scheduleReconnect()
      else if (!room.value && pending) {
        pending = null
        goToLobby('Tidak bisa terhubung ke server.')
      }
    },
  )

  function scheduleReconnect() {
    if (!reconnecting.value) {
      reconnecting.value = true
      retryCount = 0
      giveUpAt = Date.now() + RECONNECT_GIVE_UP_MS
    }
    if (Date.now() > giveUpAt) {
      goToLobby('Gagal menyambung kembali ke permainan.')
      return
    }
    const delay = Math.min(1000 * 2 ** retryCount, RECONNECT_MAX_DELAY_MS)
    retryCount++
    clearTimeout(retryTimer)
    retryTimer = window.setTimeout(reconnectNow, delay)
  }

  function reconnectNow() {
    const s = loadSession()
    if (!s) {
      goToLobby('Gagal menyambung kembali ke permainan.')
      return
    }
    clearTimeout(retryTimer)
    replaced.value = false
    reconnecting.value = true
    conn.connect(onMessage)
    sendJoin({ kind: 'rejoin', code: s.room, token: s.token })
  }

  function stopReconnect() {
    clearTimeout(retryTimer)
    reconnecting.value = false
    retryCount = 0
  }

  /** On page load: if this tab was in a game (e.g. after a refresh), go back to it. */
  function resumeSession(): boolean {
    const s = loadSession()
    if (!s) return false
    lobbyError.value = null
    conn.connect(onMessage)
    sendJoin({ kind: 'rejoin', code: s.room, token: s.token })
    return true
  }

  function enter(p: PendingJoin) {
    reset()
    lobbyError.value = null
    conn.connect(onMessage)
    sendJoin(p)
  }

  /** Solo mode: a private room with three bots, started right away (SPEC 1). */
  function startSolo(name: string, settings: Settings) {
    enter({ kind: 'create', name, code: SOLO_PREFIX + randomCode(8), solo: true, settings })
  }

  /** Make a new room for friends; the caller becomes host. */
  function createRoom(name: string, settings: Settings) {
    enter({ kind: 'create', name, code: randomCode(5), solo: false, settings })
  }

  function joinRoom(name: string, code: string) {
    enter({ kind: 'join', name, code: code.trim().toUpperCase(), solo: false })
  }

  /** The player chose to leave: give the seat up for good. */
  function leave() {
    if (room.value && conn.status === 'open') conn.send('leave_room')
    goToLobby(null)
  }

  /** Drop everything local and show the lobby (the seat is not given up). */
  function goToLobby(message: string | null) {
    pending = null
    stopReconnect()
    conn.disconnect()
    reset()
    saveSession(null)
    setURLRoom(null)
    lobbyError.value = message
  }

  function reset() {
    room.value = null
    view.value = null
    log.value = []
    dice.value = null
    error.value = null
    deadline.value = null
    replaced.value = false
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
    room, view, log, error, dice, deadline, timeLimit, lobbyError, reconnecting, replaced,
    mySeat, isHost, isSolo, legal, result, discardable,
    seatName, resultText,
    startSolo, createRoom, joinRoom, leave, goToLobby, resumeSession, reconnectNow,
    addBot, removeBot, fillWithBots, updateSettings, startGame,
    discard, act, nextRound,
  }
})
