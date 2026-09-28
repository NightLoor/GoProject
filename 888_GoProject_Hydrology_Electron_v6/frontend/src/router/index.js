import { createRouter, createWebHashHistory } from 'vue-router'
import MainLayout from '../layouts/MainLayout.vue'
import WaterRain from '../views/WaterRain.vue'
import Seepage from '../views/Seepage.vue'
import Alarms from '../views/Alarms.vue'
import History from '../views/History.vue'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/',
      component: MainLayout,
      children: [
        { path: '', redirect: '/water-rain' },
        { path: 'water-rain', component: WaterRain },
        { path: 'seepage', component: Seepage },
        { path: 'alarms', component: Alarms },
        { path: 'history', component: History },
      ],
    },
  ],
})

export default router
