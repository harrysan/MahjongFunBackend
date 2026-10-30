// Wire types for the WebSocket protocol (SPEC.md section 7).
// They mirror server/internal/engine and server/internal/room.

export type Seat = number

export interface Tile {
  id: number
  kind: number
}

export type ActionType = 'discard' | 'hu' | 'chi' | 'pung' | 'kong' | 'pass'

export interface Action {
  type: ActionType
  tile_id: number
  tile_id2: number
}

export type MeldType = 'chi' | 'pung' | 'kong' | 'concealed_kong'

export interface Meld {
  type: MeldType
  tiles: Tile[]
  from: Seat
}

export interface GameResult {
  draw: boolean
  winner: Seat
  from: Seat
  self_draw: boolean
  win_tile: Tile
}

export type Phase = 'Discarding' | 'ClaimWindow' | 'Ended'

export interface PlayerView {
  seat: Seat
  phase: Phase
  turn: Seat
  dealer: Seat
  hand: Tile[]
  drawn?: Tile
  melds: Meld[][]
  bonus: Tile[][]
  discards: Tile[][]
  hand_counts: number[]
  wall_remaining: number
  legal_actions: Action[] | null
  pending: Seat[] | null
  last_discard?: Tile
  last_discarder: Seat
  result?: GameResult
  time_left_ms: number // 0 = no time limit
  time_limit_ms: number
}

/** Room options chosen by the host. A timeout of 0 means no limit. */
export interface Settings {
  bot_delay_min_ms: number
  bot_delay_max_ms: number
  turn_seconds: number
  claim_seconds: number
}

export interface SeatInfo {
  seat: Seat
  kind: 'empty' | 'human' | 'bot'
  name: string
  connected: boolean // false: a human whose connection dropped; a bot plays for them
}

export interface RoomState {
  room: string
  status: 'waiting' | 'playing' | 'ended'
  host: Seat
  your_seat: Seat
  dealer: Seat
  seats: SeatInfo[]
  settings: Settings
  token?: string // lets this player reconnect (rejoin_room); keep it secret
}

export interface GameEvent {
  type: 'drew' | 'bonus' | 'discarded' | 'claim_window' | 'claimed' | 'turn' | 'game_end' | 'dice_rolled'
  seat: Seat
  tiles?: Tile[]
  meld?: MeldType
  result?: GameResult
  dice?: [number, number]
  new_dealer?: Seat
}

export interface ServerError {
  code: string
  message: string
}

export type ServerMessage =
  | { type: 'room_state'; data: RoomState }
  | { type: 'game_state'; data: PlayerView }
  | { type: 'event'; data: GameEvent }
  | { type: 'claim_window'; data: { tile: Tile; from: Seat; actions: Action[] } }
  | { type: 'error'; data: ServerError }
