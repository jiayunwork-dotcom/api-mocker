import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 3080,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
        configure: (proxy, options) => {
          proxy.on('proxyReq', (proxyReq, req, res) => {
            if (req.headers.upgrade && req.headers.upgrade.toLowerCase() === 'websocket') {
              proxyReq.setHeader('Connection', 'Upgrade')
              proxyReq.setHeader('Upgrade', 'websocket')
            }
          })
          proxy.on('upgrade', (req, socket, head) => {
            proxy.ws(req, socket, head)
          })
        }
      },
      '/mock': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
})
