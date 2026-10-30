import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import { useGameStore } from './game'
import type { PlayerView, RoomState, ServerMessage, Settings } from '../types'

/** Stands in for the browser WebSocket; tests drive it by hand. */
class FakeSocket {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSED = 3
  static all: FakeSocket[] = []

  readyState = FakeSocket.CONNECTING
  sent: { type: string; data: Record<string, unknown> }[] = []
  onopen: (() => void) | null = null
  onmessage: ((ev: { data: string }) => void) | null = null
  onclose: (() => void) | null = null

  constructor(public url: string) {
    FakeSocket.all.push(this)
  }
  send(m: string) {
    this.sent.push(JSON.parse(m))
  }
  close() {
    this.drop()
  }

  // Test helpers.
  open() {
    this.readyState = FakeSocket.OPEN
    this.onopen?.()
  }
  receive(m: ServerMessage) {
    this.onmessage?.({ data: JSON.stringify(m) })
  }
  drop() {
    if (this.readyState === FakeSocket.CLOSED) return
    this.readyState = FakeSocket.CLOSED
    this.onclose?.()
  }
  types() {
    return this.sent.map((m) => m.type)
  }
}

const last = () => FakeSocket.all[FakeSocket.all.length - 1]

const SETTINGS: Settings = { bot_delay_min_ms: 0, bot_delay_max_ms: 200, turn_seconds: 30, claim_seconds: 10 }

function roomState(over: Partial<RoomState> = {}): RoomState {
  return {
    room: 'ABCDE',
    status: 'waiting',
    host: 0,
    your_seat: 0,
    dealer: 0,
    seats: [0, 1, 2, 3].map((s) => ({ seat: s, kind: s === 0 ? 'human' : 'empty', name: s === 0 ? 'Ani' : '', connected: s === 0 })),
    settings: SETTINGS,
    token: 'secret-token',
    ...over,
  } as RoomState
}

function view(over: Partial<PlayerView> = {}): PlayerView {
  return {
    seat: 0,
    phase: 'Discarding',
    turn: 0,
    dealer: 0,
    hand: [
      { id: 0, kind: 0 },
      { id: 4, kind: 1 },
    ],
    melds: [[], [], [], []],
    bonus: [[], [], [], []],
    discards: [[], [], [], []],
    hand_counts: [14, 13, 13, 13],
    wall_remaining: 80,
    legal_actions: [{ type: 'discard', tile_id: 4, tile_id2: 0 }],
    pending: [0],
    last_discarder: 0,
    time_left_ms: 20_000,
    time_limit_ms: 30_000,
    ...over,
  }
}

const session = () => JSON.parse(sessionStorage.getItem('mahjong.session') ?? 'null')

beforeEach(() => {
  FakeSocket.all = []
  vi.stubGlobal('WebSocket', FakeSocket)
  vi.useFakeTimers()
  sessionStorage.clear()
  history.replaceState(null, '', '/')
  setActivePinia(createPinia())
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('entering a room', () => {
  it('starts a solo game: create, settings, three bots, start', () => {
    const game = useGameStore()
    game.startSolo('Ani', SETTINGS)
    const ws = last()
    expect(ws.url).toMatch(/\/ws$/)
    expect(ws.sent).toEqual([]) // queued until the socket opens
    ws.open()
    expect(ws.sent[0]).toMatchObject({ type: 'join_room', data: { name: 'Ani', create: true } })
    expect(String(ws.sent[0].data.room)).toMatch(/^SOLO-[A-Z2-9]{8}$/)

    ws.receive({ type: 'room_state', data: roomState({ room: String(ws.sent[0].data.room) }) })
    expect(ws.types()).toEqual(['join_room', 'update_settings', 'add_bot', 'add_bot', 'add_bot', 'start_game'])
    expect(ws.sent.filter((m) => m.type === 'add_bot').map((m) => m.data.seat)).toEqual([1, 2, 3])
    expect(game.isSolo).toBe(true)
    expect(session()).toEqual({ room: ws.sent[0].data.room, token: 'secret-token' })
    // Solo rooms are private: no invite code in the address bar.
    expect(location.search).toBe('')
  })

  it('puts a friends room code in the address bar', () => {
    const game = useGameStore()
    game.createRoom('Ani', SETTINGS)
    const ws = last()
    ws.open()
    ws.receive({ type: 'room_state', data: roomState() })
    expect(ws.types()).toEqual(['join_room', 'update_settings'])
    expect(new URL(location.href).searchParams.get('room')).toBe('ABCDE')
    expect(game.isHost).toBe(true)
  })

  it('tries another code when the random one is taken', () => {
    const game = useGameStore()
    game.createRoom('Ani', SETTINGS)
    const ws = last()
    ws.open()
    const first = ws.sent[0].data.room
    ws.receive({ type: 'error', data: { code: 'room_exists', message: 'x' } })
    expect(ws.sent[1].type).toBe('join_room')
    expect(ws.sent[1].data.room).not.toBe(first)
    expect(game.lobbyError).toBeNull()
  })

  it('goes back to the lobby when joining fails', () => {
    const game = useGameStore()
    game.joinRoom('Ani', ' abcde ')
    const ws = last()
    ws.open()
    expect(ws.sent[0]).toMatchObject({ type: 'join_room', data: { room: 'ABCDE', create: false } })
    ws.receive({ type: 'error', data: { code: 'room_not_found', message: 'Room tidak ditemukan.' } })
    expect(game.lobbyError).toBe('Room tidak ditemukan.')
    expect(game.room).toBeNull()
    expect(ws.readyState).toBe(FakeSocket.CLOSED)
  })

  it('reports an unreachable server', async () => {
    const game = useGameStore()
    game.createRoom('Ani', SETTINGS)
    last().drop()
    await nextTick()
    expect(game.lobbyError).toBe('Tidak bisa terhubung ke server.')
  })
})

describe('playing', () => {
  function inGame() {
    const game = useGameStore()
    game.createRoom('Ani', SETTINGS)
    const ws = last()
    ws.open()
    ws.receive({ type: 'room_state', data: roomState({ status: 'playing' }) })
    ws.sent = []
    return { game, ws }
  }

  it('keeps the game view and the turn clock', () => {
    const { game, ws } = inGame()
    ws.receive({ type: 'game_state', data: view() })
    expect(game.view?.wall_remaining).toBe(80)
    expect(game.deadline).toBe(Date.now() + 20_000)
    expect(game.timeLimit).toBe(30_000)
    ws.receive({ type: 'game_state', data: view({ time_left_ms: 0 }) })
    expect(game.deadline).toBeNull()
  })

  it('only sends discards the server allows', () => {
    const { game, ws } = inGame()
    ws.receive({ type: 'game_state', data: view() })
    game.discard(0) // not in legal_actions
    game.discard(4)
    expect(ws.sent).toEqual([{ type: 'discard', data: { tile_id: 4 } }])
  })

  it('sends claims during a claim window and own actions otherwise', () => {
    const { game, ws } = inGame()
    ws.receive({ type: 'game_state', data: view({ phase: 'ClaimWindow' }) })
    game.act({ type: 'chi', tile_id: 0, tile_id2: 4 })
    game.act({ type: 'pass', tile_id: 0, tile_id2: 0 })
    ws.receive({ type: 'game_state', data: view() })
    game.act({ type: 'kong', tile_id: 4, tile_id2: 0 })
    game.act({ type: 'hu', tile_id: 0, tile_id2: 0 })
    expect(ws.sent).toEqual([
      { type: 'claim', data: { action: 'chi', tiles: [0, 4] } },
      { type: 'claim', data: { action: 'pass' } },
      { type: 'self_action', data: { action: 'kong', tile_id: 4 } },
      { type: 'self_action', data: { action: 'hu' } },
    ])
  })

  it('logs events and shows the dice', () => {
    const { game, ws } = inGame()
    ws.receive({ type: 'event', data: { type: 'discarded', seat: 1, tiles: [{ id: 8, kind: 2 }] } })
    ws.receive({ type: 'event', data: { type: 'dice_rolled', seat: 0, dice: [3, 4], new_dealer: 2 } })
    expect(game.log).toEqual(['Kursi 2 membuang 3 Titik', 'Kamu melempar dadu: 3 + 4 = 7. Dealer baru: Kursi 3'])
    expect(game.dice).toEqual({ dice: [3, 4], newDealer: 2 })
    vi.advanceTimersByTime(4500)
    expect(game.dice).toBeNull()
  })

  it('calls out claims and wins over the seat', () => {
    const { game, ws } = inGame()
    ws.receive({ type: 'event', data: { type: 'claimed', seat: 2, meld: 'pung', tiles: [] } })
    expect(game.callout).toMatchObject({ seat: 2, text: 'Pung!' })
    const win = { draw: false, winner: 3, from: 1, self_draw: false, win_tile: { id: 1, kind: 1 } }
    ws.receive({ type: 'event', data: { type: 'game_end', seat: 3, result: win } })
    expect(game.callout).toMatchObject({ seat: 3, text: 'Hu!' })
    vi.advanceTimersByTime(1600)
    expect(game.callout).toBeNull()
  })

  it('keeps the player hand order until the next round', () => {
    const { game, ws } = inGame()
    ws.receive({ type: 'game_state', data: view() })
    game.arrangeHand([4, 0])
    expect(game.handOrder).toEqual([4, 0])
    ws.receive({ type: 'game_state', data: view({ wall_remaining: 79 }) })
    expect(game.handOrder).toEqual([4, 0])
    game.sortHand()
    expect(game.handOrder).toBeNull()

    game.arrangeHand([4, 0])
    ws.receive({ type: 'game_state', data: view({ phase: 'Ended' }) })
    ws.receive({ type: 'game_state', data: view() }) // a new deal: ids mean other tiles now
    expect(game.handOrder).toBeNull()
  })

  it('shows server errors for a few seconds', () => {
    const { game, ws } = inGame()
    ws.receive({ type: 'error', data: { code: 'rate_limited', message: 'Terlalu banyak aksi.' } })
    expect(game.error).toBe('Terlalu banyak aksi.')
    vi.advanceTimersByTime(3000)
    expect(game.error).toBeNull()
  })

  it('leaving gives the seat up and forgets the session', () => {
    const { game, ws } = inGame()
    game.leave()
    expect(ws.types()).toEqual(['leave_room'])
    expect(session()).toBeNull()
    expect(game.room).toBeNull()
  })

  describe('connection loss', () => {
    it('reconnects with the token and gets the seat back', async () => {
      const { game, ws } = inGame()
      ws.drop()
      await nextTick()
      expect(game.reconnecting).toBe(true)
      expect(game.room).not.toBeNull() // the table stays on screen

      vi.advanceTimersByTime(1000)
      const ws2 = last()
      expect(ws2).not.toBe(ws)
      ws2.open()
      expect(ws2.sent).toEqual([{ type: 'rejoin_room', data: { room: 'ABCDE', token: 'secret-token' } }])
      ws2.receive({ type: 'room_state', data: roomState({ status: 'playing' }) })
      expect(game.reconnecting).toBe(false)
      expect(game.log).toContain('— Tersambung kembali —')
    })

    it('backs off between attempts', async () => {
      const { ws } = inGame()
      ws.drop()
      await nextTick()
      vi.advanceTimersByTime(1000)
      expect(FakeSocket.all).toHaveLength(2)
      await nextTick() // a browser reports the failure in a later task
      last().drop() // first retry fails
      await nextTick()
      vi.advanceTimersByTime(1999)
      expect(FakeSocket.all).toHaveLength(2)
      vi.advanceTimersByTime(1)
      expect(FakeSocket.all).toHaveLength(3)
    })

    it('stops when the game is gone', async () => {
      const { game, ws } = inGame()
      ws.drop()
      await nextTick()
      vi.advanceTimersByTime(1000)
      last().open()
      last().receive({ type: 'error', data: { code: 'bad_token', message: 'x' } })
      expect(game.lobbyError).toBe('Permainan sebelumnya sudah berakhir.')
      expect(game.room).toBeNull()
      expect(session()).toBeNull()
    })

    it('does not fight another tab for the seat', async () => {
      const { game, ws } = inGame()
      ws.receive({ type: 'error', data: { code: 'session_moved', message: 'x' } })
      ws.drop()
      await nextTick()
      expect(game.replaced).toBe(true)
      expect(game.reconnecting).toBe(false)
      vi.advanceTimersByTime(60_000)
      expect(FakeSocket.all).toHaveLength(1)
    })
  })
})

describe('after a refresh', () => {
  it('rejoins the room this tab was in', () => {
    sessionStorage.setItem('mahjong.session', JSON.stringify({ room: 'ABCDE', token: 't1' }))
    const game = useGameStore()
    expect(game.resumeSession()).toBe(true)
    last().open()
    expect(last().sent).toEqual([{ type: 'rejoin_room', data: { room: 'ABCDE', token: 't1' } }])
  })

  it('does nothing without a saved session', () => {
    sessionStorage.setItem('mahjong.session', 'garbage{')
    expect(useGameStore().resumeSession()).toBe(false)
    expect(FakeSocket.all).toHaveLength(0)
  })
})
