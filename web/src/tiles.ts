// Display helpers for tile kinds (see SPEC.md section 5 for the numbering).

// Tile images in src/assets/tiles, named the usual way:
// 1-9 + m (characters) / p (dots) / s (bamboo),
// z1-z4 East South West North, z5 Red, z6 Green, z7 White,
// f1-f4 seasons, f5-f8 flowers.
const IMAGES = import.meta.glob<string>('./assets/tiles/*.png', { eager: true, import: 'default' })

const SUIT_LETTERS = ['p', 's', 'm'] // our suit order: dots, bamboo, characters
const SUIT_NAMES = ['Titik', 'Bambu', 'Karakter']
const HONOR_NAMES = ['Angin Timur', 'Angin Selatan', 'Angin Barat', 'Angin Utara', 'Naga Merah', 'Naga Hijau', 'Naga Putih']

export const WIND_NAMES = ['Timur', 'Selatan', 'Barat', 'Utara']

function imageFile(kind: number): string {
  if (kind < 27) return `${(kind % 9) + 1}${SUIT_LETTERS[Math.floor(kind / 9)]}`
  if (kind < 34) return `z${kind - 27 + 1}`
  if (kind < 38) return `f${kind - 34 + 5}` // flowers
  return `f${kind - 38 + 1}` // seasons
}

export function tileImage(kind: number): string | undefined {
  return IMAGES[`./assets/tiles/${imageFile(kind)}.png`]
}

export function tileName(kind: number): string {
  if (kind < 27) return `${(kind % 9) + 1} ${SUIT_NAMES[Math.floor(kind / 9)]}`
  if (kind < 34) return HONOR_NAMES[kind - 27]
  if (kind < 38) return `Bunga ${kind - 34 + 1}`
  return `Musim ${kind - 38 + 1}`
}

/** Wind of a seat in this round: the dealer is East. */
export function seatWind(seat: number, dealer: number): string {
  return WIND_NAMES[(seat - dealer + 4) % 4]
}
