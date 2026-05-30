import { createApp } from 'vue'
import App from './App.vue'
import router from '@/router'
import { i18n } from '@/i18n'
import 'vue-sonner/style.css'
import '@/styles/global.css'

const app = createApp(App)
app.use(i18n)
app.use(router)

app.mount('#app')
