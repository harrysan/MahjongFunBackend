import { devices, expect, test, type Page } from '@playwright/test'

// Table features from M8: arranging the hand, phones, sound toggle, PWA.

/** Solo game with slow bots: the human is dealer in round one and moves first. */
async function soloOnMyTurn(page: Page) {
  await page.goto('/')
  await page.getByLabel('Nama kamu').fill('Ani')
  await page.locator('#lobby-bot').selectOption('4') // Lambat
  await page.getByRole('button', { name: 'Main Solo (vs 3 Bot)' }).click()
  await expect(page.locator('.hand button.tile')).toHaveCount(14)
}

/** A device profile without defaultBrowserType, which test.use cannot set inside a describe. */
function phone(name: string) {
  const { defaultBrowserType: _, ...rest } = devices[name]
  return rest
}

const slotIds = (page: Page) =>
  page.locator('.hand .slot').evaluateAll((els) => els.map((e) => Number((e as HTMLElement).dataset.tileId)))

async function center(page: Page, selector: string) {
  const b = (await page.locator(selector).boundingBox())!
  return { x: b.x + b.width / 2, y: b.y + b.height / 2 }
}

test('drag a tile to rearrange the hand, then sort it again', async ({ page }) => {
  await soloOnMyTurn(page)
  const before = await slotIds(page)

  // Drag the first tile onto the fifth.
  const from = await center(page, '.hand .slot >> nth=0')
  const to = await center(page, '.hand .slot >> nth=4')
  await page.mouse.move(from.x, from.y)
  await page.mouse.down()
  await page.mouse.move(to.x, to.y, { steps: 12 })
  await page.mouse.up()

  const after = await slotIds(page)
  expect(after.slice(0, 5)).toEqual([...before.slice(1, 5), before[0]])
  expect(after.slice(5)).toEqual(before.slice(5))
  // Dragging is not discarding.
  await expect(page.locator('.hand button.tile')).toHaveCount(14)
  await expect(page.getByText('Kamu membuang')).toHaveCount(0)

  await page.getByRole('button', { name: 'Urutkan' }).click()
  expect(await slotIds(page)).toEqual(before)
  await expect(page.getByRole('button', { name: 'Urutkan' })).toBeHidden()
})

test('the sound setting is remembered', async ({ page }) => {
  await soloOnMyTurn(page)
  await page.getByRole('button', { name: 'Matikan suara' }).click()
  await expect(page.getByRole('button', { name: 'Nyalakan suara' })).toBeVisible()
  await page.reload()
  await expect(page.getByRole('button', { name: 'Nyalakan suara' })).toBeVisible()
})

test('installable: manifest and service worker', async ({ page }) => {
  await page.goto('/')
  const href = await page.locator('link[rel=manifest]').getAttribute('href')
  const res = await page.request.get(href!)
  expect(res.ok()).toBe(true)
  expect(res.headers()['content-type']).toContain('application/manifest+json')
  const manifest = await res.json()
  expect(manifest.display).toBe('standalone')
  expect(manifest.icons.map((i: { sizes: string }) => i.sizes)).toEqual(expect.arrayContaining(['192x192', '512x512']))
  for (const icon of manifest.icons) expect((await page.request.get(icon.src)).ok()).toBe(true)
  const state = await page.evaluate(async () => (await navigator.serviceWorker.ready).active?.state)
  expect(state).toBe('activated')
})

test.describe('phone, portrait', () => {
  test.use(phone('Pixel 7'))

  test('fits the screen, and a tile is discarded only on the second tap', async ({ page }) => {
    await soloOnMyTurn(page)
    const fits = await page.evaluate(() => document.scrollingElement!.scrollHeight <= innerHeight + 1)
    expect(fits).toBe(true)

    const drawn = page.locator('.hand .slot.drawn button.tile')
    await drawn.tap()
    await expect(page.getByText('Ketuk lagi untuk membuang')).toBeVisible()
    await expect(page.locator('.hand button.tile')).toHaveCount(14)
    await drawn.tap()
    await expect(page.getByText('Kamu membuang')).toBeVisible()
  })
})

test.describe('phone, landscape', () => {
  test.use(phone('Pixel 7 landscape'))

  test('the whole table fits without scrolling', async ({ page }) => {
    await soloOnMyTurn(page)
    const fits = await page.evaluate(() => document.scrollingElement!.scrollHeight <= innerHeight + 1)
    expect(fits).toBe(true)
    const hand = (await page.locator('.hand').boundingBox())!
    const viewport = page.viewportSize()!
    expect(hand.y + hand.height).toBeLessThanOrEqual(viewport.height)
    expect(hand.x + hand.width).toBeLessThanOrEqual(viewport.width)
  })
})
