import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiTarget = env.VITE_API_TARGET || 'http://127.0.0.1:8080'
  const devPort = Number(env.VITE_DEV_PORT || 5173)

  return {
    plugins: [react(), tailwindcss()],
    server: {
      host: env.VITE_DEV_HOST || '0.0.0.0',
      port: devPort,
      proxy: {
        '/api': {
          target: apiTarget,
          changeOrigin: true,
        },
        '/ws': {
          target: apiTarget,
          ws: true,
          changeOrigin: true,
        },
      },
    },
    preview: {
      host: env.VITE_DEV_HOST || '0.0.0.0',
      port: Number(env.VITE_PREVIEW_PORT || 4173),
    },
    build: {
      outDir: 'dist',
      rollupOptions: {
        output: {
          manualChunks(id: string) {
            if (id.includes('/node_modules/chart.js/') || id.includes('/node_modules/react-chartjs-2/')) {
              return 'charts'
            }
            if (id.includes('/node_modules/lucide-react/')) {
              return 'icons'
            }
            if (id.includes('/node_modules/react/') || id.includes('/node_modules/react-dom/') || id.includes('/node_modules/react-router')) {
              return 'react'
            }
          },
        },
      },
    },
    base: '/',
  }
})
