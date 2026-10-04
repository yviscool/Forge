import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import StudentLobby from './views/StudentLobby.vue'
import TeacherConsole from './views/TeacherConsole.vue'
import './style.css'
import 'katex/dist/katex.min.css'

const router = createRouter({
  history: createWebHistory('/app/'),
  routes: [
    { path: '/', component: StudentLobby },
    { path: '/teacher', component: TeacherConsole },
  ],
})

createApp(App).use(router).mount('#app')
