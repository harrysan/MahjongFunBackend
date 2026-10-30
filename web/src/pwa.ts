// "Install app" support. Chrome and Edge fire beforeinstallprompt when the
// app can be installed; we keep the event and show our own button.
// Safari has no such event: there the player uses Share > Add to Home Screen.
import { ref } from 'vue'

interface InstallPromptEvent extends Event {
  prompt(): Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

const promptEvent = ref<InstallPromptEvent | null>(null)
/** True when the browser lets us offer installation. */
export const canInstall = ref(false)

window.addEventListener('beforeinstallprompt', (e) => {
  e.preventDefault()
  promptEvent.value = e as InstallPromptEvent
  canInstall.value = true
})
window.addEventListener('appinstalled', () => {
  promptEvent.value = null
  canInstall.value = false
})

export async function installApp() {
  const e = promptEvent.value
  if (!e) return
  promptEvent.value = null
  canInstall.value = false
  await e.prompt()
}
