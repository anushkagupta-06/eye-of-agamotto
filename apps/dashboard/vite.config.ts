import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'
import path from 'path'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  // Load .env from root workspace directory
  const rootEnvDir = path.resolve(import.meta.dirname || '.', '../../')
  const rootEnv = loadEnv(mode, rootEnvDir, '')
  const geminiKey =
    rootEnv.VITE_GEMINI_API_KEY ||
    rootEnv.GEMINI_API_KEY_DEFAULT ||
    rootEnv.GEMINI_API_KEY ||
    ''

  return {
    plugins: [react()],
    envDir: '../../',
    define: {
      'import.meta.env.VITE_GEMINI_API_KEY': JSON.stringify(geminiKey),
      'import.meta.env.GEMINI_API_KEY_DEFAULT': JSON.stringify(geminiKey),
      'process.env.GEMINI_API_KEY_DEFAULT': JSON.stringify(geminiKey),
    },
    server: {
      proxy: {
        '/api/query': {
          target: 'http://127.0.0.1:8082',
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/api\/query/, ''),
        },
        '/api/mcp': {
          target: 'http://127.0.0.1:8083',
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/api\/mcp/, ''),
        },
        '/api/clickhouse': {
          target: 'http://127.0.0.1:8123',
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/api\/clickhouse/, ''),
        },
        '/api/gateway': {
          target: 'http://127.0.0.1:4318',
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/api\/gateway/, ''),
        },
      },
    },
  }
})
