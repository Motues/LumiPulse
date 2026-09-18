import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './style.css'
// 注册文案并初始化语言（localStorage → 浏览器语言 → 默认中文），
// 必须在挂载前完成，否则首屏会先渲染出默认语言再切换
import './locales/zh-CN'
import './locales/en-US'
import { initLocale } from './composables/useI18n'

initLocale()

const app = createApp(App)
app.use(router)
app.mount('#app')
