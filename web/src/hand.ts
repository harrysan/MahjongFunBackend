// The player's own arrangement of their hand. It is display only: the server
// keeps the real hand, and sends it sorted.
import type { Tile } from './types'

export interface ArrangedHand {
  tiles: Tile[] // in screen order
  drawn?: Tile // the just-drawn tile, shown apart until the player places it
}

/**
 * Lays out the hand. order lists tile ids the way the player arranged them,
 * or is null for the server's sorted order. Tiles the player has not placed
 * yet (new from a claim) go to the end; the drawn tile stays apart.
 */
export function arrangeHand(hand: Tile[], drawn: Tile | undefined, order: number[] | null): ArrangedHand {
  const byId = new Map(hand.map((t) => [t.id, t]))
  const tiles: Tile[] = []
  for (const id of order ?? []) {
    const t = byId.get(id)
    if (t) {
      tiles.push(t)
      byId.delete(id)
    }
  }
  const apart = drawn && byId.has(drawn.id) ? drawn : undefined
  for (const t of hand) {
    if (byId.has(t.id) && t.id !== apart?.id) tiles.push(t)
  }
  return { tiles, drawn: apart }
}

/** Moves id to where target is now (or to the end when target is not in order). */
export function moveTile(order: number[], id: number, target: number | null): number[] {
  const from = order.indexOf(id)
  const out = order.filter((x) => x !== id)
  const i = target === null ? -1 : out.indexOf(target)
  if (i < 0) {
    out.push(id)
    return out
  }
  // Dragging right puts the tile after the one under the pointer, left before it.
  const rightward = from !== -1 && from < order.indexOf(target!)
  out.splice(rightward ? i + 1 : i, 0, id)
  return out
}
