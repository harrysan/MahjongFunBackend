import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './style.css'
import './pwa'

createApp(App).use(createPinia()).mount('#app')

// Installable app (PWA). Not in dev: a cached page would hide Vite's hot reload.
if (import.meta.env.PROD && 'serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').catch((err) => console.warn('service worker:', err))
  })
}
