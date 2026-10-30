import { expect, test, type Page } from '@playwright/test'

/** Fills in the lobby: name and bot speed (index into BOT_SPEEDS: 0 = Kilat, 4 = Lambat). */
async function lobby(page: Page, name: string, botSpeed = 0, path = '/') {
  await page.goto(path)
  await page.getByLabel('Nama kamu').fill(name)
  await page.locator('#lobby-bot').selectOption(String(botSpeed))
}

const resultDialog = (page: Page) => page.locator('.overlay .dialog')
const handTiles = (page: Page) => page.locator('.hand .tile')

/**
 * Plays like a very simple human until the round ends: always takes a hu,
 * passes every other claim, and throws the drawn tile (or the last one).
 */
async function playRound(page: Page) {
  const deadline = Date.now() + 100_000
  while (Date.now() < deadline) {
    if (await resultDialog(page).isVisible()) return
    const hu = page.getByRole('button', { name: 'Hu!' })
    const pass = page.getByRole('button', { name: 'Pass' })
    const playable = page.locator('.hand button.tile')
    try {
      if (await hu.isVisible()) await hu.click({ timeout: 1000 })
      else if (await pass.isVisible()) await pass.click({ timeout: 1000 })
      else if ((await playable.count()) > 0) await playable.last().click({ timeout: 1000 })
    } catch {
      // The table changed under the click (a bot moved); look again.
    }
    await page.waitForTimeout(50)
  }
  throw new Error('round did not end in time')
}

async function expectRoundResult(page: Page) {
  await expect(resultDialog(page).getByRole('heading')).toHaveText(/Kamu menang!|Ronde selesai|Seri/)
}

test('solo: play a full round, then start the next one', async ({ page }) => {
  await lobby(page, 'Ani')
  await page.getByRole('button', { name: 'Main Solo (vs 3 Bot)' }).click()
  await expect(page.getByText('Sisa wall')).toBeVisible()
  await expect(page.locator('.seat .tag', { hasText: 'bot' })).toHaveCount(3)

  await playRound(page)
  await expectRoundResult(page)
  const draw = (await resultDialog(page).getByRole('heading').textContent()) === 'Seri'

  await page.getByRole('button', { name: 'Ronde berikutnya' }).click()
  await expect(resultDialog(page)).toBeHidden()
  await expect(page.getByText('— Ronde baru —')).toBeVisible()
  // After a win the winner rolls the dice for the next dealer (SPEC 4.7).
  if (!draw) await expect(page.locator('.dice-roll')).toContainText('Dealer baru')
})

test('multiplayer: two players and two bots finish a round', async ({ browser }) => {
  const hostCtx = await browser.newContext()
  const guestCtx = await browser.newContext()
  const host = await hostCtx.newPage()
  const guest = await guestCtx.newPage()

  await lobby(host, 'Host')
  await host.getByRole('button', { name: 'Buat Room untuk Teman' }).click()
  const code = (await host.locator('.code').textContent())!.trim()
  expect(code).toMatch(/^[A-Z2-9]{5}$/)

  // The guest opens the invite link.
  await lobby(guest, 'Tamu', 0, `/?room=${code}`)
  await expect(guest.getByText(`Kamu diundang ke room ${code}`)).toBeVisible()
  await guest.locator('form.highlight').getByRole('button', { name: 'Gabung' }).click()
  await expect(guest.getByText('Menunggu host memulai permainan…')).toBeVisible()
  await expect(host.locator('.seats')).toContainText('Tamu')

  await host.getByRole('button', { name: 'Isi kursi kosong dengan bot' }).click()
  await host.getByRole('button', { name: 'Mulai Permainan' }).click()
  await expect(host.getByText(`Room ${code}`)).toBeVisible()
  await expect(guest.getByText(`Room ${code}`)).toBeVisible()

  await Promise.all([playRound(host), playRound(guest)])
  await expectRoundResult(host)
  await expectRoundResult(guest)
  await expect(guest.getByText('Menunggu host…')).toBeVisible()
  await expect(host.getByRole('button', { name: 'Ronde berikutnya' })).toBeVisible()

  await hostCtx.close()
  await guestCtx.close()
})

test('refresh in the middle of a round returns to the same seat and hand', async ({ page }) => {
  // Slow bots: the human is dealer in round one and moves first, so nothing
  // changes while the page reloads.
  await lobby(page, 'Ani', 4)
  await page.getByRole('button', { name: 'Main Solo (vs 3 Bot)' }).click()
  await expect(page.locator('.hand button.tile').first()).toBeVisible()
  const before = await handTiles(page).evaluateAll((els) => els.map((e) => e.getAttribute('title')))
  expect(before).toHaveLength(14)

  await page.reload()
  await expect(page.locator('.hand button.tile').first()).toBeVisible()
  const after = await handTiles(page).evaluateAll((els) => els.map((e) => e.getAttribute('title')))
  expect(after).toEqual(before)
  await expect(page.getByText('Sisa wall')).toBeVisible()

  // Still playable: the discard goes through.
  await page.locator('.hand button.tile').last().click()
  await expect(page.getByText('Kamu membuang')).toBeVisible()
})

test('a dropped connection shows the reconnect screen and recovers', async ({ page }) => {
  // Every socket goes through this route, so the test can cut the current one.
  let cut: (() => void) | undefined
  await page.routeWebSocket(/\/ws$/, (ws) => {
    const server = ws.connectToServer()
    cut = () => {
      server.close()
      ws.close()
    }
  })

  await lobby(page, 'Ani', 4)
  await page.getByRole('button', { name: 'Main Solo (vs 3 Bot)' }).click()
  await expect(page.locator('.hand button.tile').first()).toBeVisible()

  cut!()
  await expect(page.getByRole('alertdialog', { name: 'Koneksi terputus' })).toBeVisible()
  // The first retry comes after a second and gets the seat back.
  await expect(page.getByRole('alertdialog')).toBeHidden({ timeout: 15_000 })
  await expect(page.getByText('— Tersambung kembali —')).toBeVisible()
  await page.locator('.hand button.tile').last().click()
  await expect(page.getByText('Kamu membuang')).toBeVisible()
})
