// Table sounds, synthesized with the Web Audio API (no sound files to load).
import { ref } from 'vue'

export type SoundName = 'discard' | 'claim' | 'turn' | 'win' | 'end' | 'dice'

const MUTE_KEY = 'mahjong.muted'

function loadMuted(): boolean {
  try {
    return localStorage.getItem(MUTE_KEY) === '1'
  } catch {
    return false
  }
}

export const muted = ref(loadMuted())

export function setMuted(m: boolean) {
  muted.value = m
  try {
    localStorage.setItem(MUTE_KEY, m ? '1' : '0')
  } catch {
    // not remembered; fine
  }
}

let ctx: AudioContext | null = null

function audio(): AudioContext | null {
  if (muted.value) return null
  const AC = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
  if (!AC) return null
  try {
    ctx ??= new AC()
  } catch {
    return null
  }
  if (ctx.state === 'suspended') void ctx.resume()
  return ctx
}

/** A tone that fades out. at is seconds from now. */
function tone(a: AudioContext, freq: number, at: number, dur: number, gain: number, type: OscillatorType = 'sine') {
  const t0 = a.currentTime + at
  const osc = a.createOscillator()
  const g = a.createGain()
  osc.type = type
  osc.frequency.setValueAtTime(freq, t0)
  g.gain.setValueAtTime(0.0001, t0)
  g.gain.exponentialRampToValueAtTime(gain, t0 + 0.01)
  g.gain.exponentialRampToValueAtTime(0.0001, t0 + dur)
  osc.connect(g).connect(a.destination)
  osc.start(t0)
  osc.stop(t0 + dur + 0.02)
}

/** A short filtered noise burst: the "clack" of a tile. */
function clack(a: AudioContext, at: number, gain: number, freq = 1800) {
  const t0 = a.currentTime + at
  const len = Math.floor(a.sampleRate * 0.05)
  const buf = a.createBuffer(1, len, a.sampleRate)
  const data = buf.getChannelData(0)
  for (let i = 0; i < len; i++) data[i] = (Math.random() * 2 - 1) * (1 - i / len) ** 3
  const src = a.createBufferSource()
  src.buffer = buf
  const f = a.createBiquadFilter()
  f.type = 'bandpass'
  f.frequency.value = freq
  f.Q.value = 1.2
  const g = a.createGain()
  g.gain.value = gain
  src.connect(f).connect(g).connect(a.destination)
  src.start(t0)
}

export function playSound(name: SoundName) {
  const a = audio()
  if (!a) return
  switch (name) {
    case 'discard':
      clack(a, 0, 0.9)
      tone(a, 320, 0, 0.08, 0.08, 'triangle')
      break
    case 'claim':
      clack(a, 0, 0.7, 1400)
      tone(a, 660, 0, 0.12, 0.12, 'triangle')
      tone(a, 990, 0.09, 0.18, 0.12, 'triangle')
      break
    case 'turn':
      tone(a, 880, 0, 0.35, 0.07)
      tone(a, 1320, 0.08, 0.4, 0.05)
      break
    case 'win':
      ;[523, 659, 784, 1047].forEach((f, i) => tone(a, f, i * 0.11, 0.45, 0.12, 'triangle'))
      break
    case 'end':
      tone(a, 440, 0, 0.3, 0.1, 'triangle')
      tone(a, 330, 0.18, 0.45, 0.1, 'triangle')
      break
    case 'dice':
      for (let i = 0; i < 7; i++) clack(a, i * 0.07 + Math.random() * 0.03, 0.5, 2400 + Math.random() * 800)
      break
  }
}
