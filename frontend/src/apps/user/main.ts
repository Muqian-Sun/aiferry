import App from '@/App.vue'
import { bootstrapApp } from '@/app/bootstrap'
import { setSiteContext } from '@/app/siteContext'
import router, { userCustomMenuItems } from './router'

setSiteContext({ router, getCustomMenuItems: userCustomMenuItems })
void bootstrapApp(App, router)
