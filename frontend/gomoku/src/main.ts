import { createApp } from 'vue'
import ui from '@nuxt/ui/vue-plugin'
import { createRouter, createWebHashHistory } from 'vue-router'
import App from './App.vue'
import './style.css'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [{ path: '/', component: { template: '<div></div>' } }]
})

const app = createApp(App)
app.use(router)
app.use(ui)
app.mount('#app')
