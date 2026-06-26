import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    watch: {
      ignored: [
        '**/node_modules/**',
        '**/dist/**',
        '**/wailsjs/**',
        '../.git/**',
        '../build/**',
        '../ext-project-refs/**',
        '../internal/**',
      ],
    },
  },
})
