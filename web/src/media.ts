import { onUnmounted, ref, type Ref } from 'vue'

/** A reactive window.matchMedia(query).matches, for use in components. */
export function useMedia(query: string): Ref<boolean> {
  const mq = typeof window.matchMedia === 'function' ? window.matchMedia(query) : null
  const matches = ref(mq?.matches ?? false)
  const update = () => (matches.value = mq!.matches)
  mq?.addEventListener('change', update)
  onUnmounted(() => mq?.removeEventListener('change', update))
  return matches
}

/** Phones: narrow, or short in landscape. Matches the CSS breakpoints in style.css. */
export const COMPACT_QUERY = '(max-width: 720px), (max-height: 500px) and (orientation: landscape)'
/** Touch screens, where a tap is easy to make by mistake. */
export const TOUCH_QUERY = '(pointer: coarse)'
