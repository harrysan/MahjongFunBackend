import { describe, expect, it } from 'vitest'
import { arrangeHand, moveTile } from './hand'

const t = (id: number) => ({ id, kind: id })
const ids = (ts: { id: number }[]) => ts.map((x) => x.id)

describe('arrangeHand', () => {
  const hand = [t(1), t(2), t(3), t(9)]

  it('uses the server order and keeps the drawn tile apart', () => {
    const a = arrangeHand(hand, t(9), null)
    expect(ids(a.tiles)).toEqual([1, 2, 3])
    expect(a.drawn?.id).toBe(9)
  })

  it('follows the player order and appends tiles not placed yet', () => {
    const a = arrangeHand([...hand, t(5)], t(9), [3, 1, 7])
    expect(ids(a.tiles)).toEqual([3, 1, 2, 5]) // 7 left the hand; 2 and 5 are new
    expect(a.drawn?.id).toBe(9)
  })

  it('puts the drawn tile inline once the player placed it', () => {
    const a = arrangeHand(hand, t(9), [9, 3, 2, 1])
    expect(ids(a.tiles)).toEqual([9, 3, 2, 1])
    expect(a.drawn).toBeUndefined()
  })
})

describe('moveTile', () => {
  it('moves right: lands after the target', () => {
    expect(moveTile([1, 2, 3, 4], 1, 3)).toEqual([2, 3, 1, 4])
  })
  it('moves left: lands before the target', () => {
    expect(moveTile([1, 2, 3, 4], 4, 2)).toEqual([1, 4, 2, 3])
  })
  it('adds a tile that was not in the order (the drawn one)', () => {
    expect(moveTile([1, 2, 3], 9, 2)).toEqual([1, 9, 2, 3])
    expect(moveTile([1, 2, 3], 9, null)).toEqual([1, 2, 3, 9])
  })
})
