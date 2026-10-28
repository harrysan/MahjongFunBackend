import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { ServerMessage } from '../types'

export type ConnectionStatus = 'idle' | 'connecting' | 'open' | 'closed'

function socketURL(): string {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${location.host}/ws`
}

/** The WebSocket to the server. Messages sent before the socket opens are queued. */
export const useConnectionStore = defineStore('connection', () => {
  const status = ref<ConnectionStatus>('idle')
  let socket: WebSocket | null = null
  let queue: string[] = []

  function connect(onMessage: (m: ServerMessage) => void) {
    disconnect()
    status.value = 'connecting'
    const ws = new WebSocket(socketURL())
    socket = ws
    ws.onopen = () => {
      status.value = 'open'
      for (const m of queue) ws.send(m)
      queue = []
    }
    ws.onmessage = (ev) => {
      try {
        onMessage(JSON.parse(ev.data as string) as ServerMessage)
      } catch (err) {
        console.error('bad server message', err)
      }
    }
    ws.onclose = () => {
      if (socket === ws) {
        status.value = 'closed'
        socket = null
      }
    }
  }

  function send(type: string, data: object = {}) {
    const msg = JSON.stringify({ type, data })
    if (socket && socket.readyState === WebSocket.OPEN) socket.send(msg)
    else queue.push(msg)
  }

  function disconnect() {
    queue = []
    if (socket) {
      const ws = socket
      socket = null
      ws.close()
    }
    status.value = 'idle'
  }

  return { status, connect, send, disconnect }
})
