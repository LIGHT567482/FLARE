import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', 'FLARE_')
  return {
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: env.FLARE_BRIDGE_URL || 'https://127.0.0.1:8443',
        changeOrigin: true,
        secure: false,
        rewrite: path => path.replace(/^\/api/, ''),
        configure: proxy => {
          proxy.on('proxyReq', proxyReq => {
            if (env.FLARE_BRIDGE_TOKEN) {
              proxyReq.setHeader('Authorization', `Bearer ${env.FLARE_BRIDGE_TOKEN}`)
            }
          })
        },
      },
    },
  },
  }
})
