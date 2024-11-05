import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react()
  ],
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
  },
  base: './',
  logLevel: 'info',
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
        ws: true,
        configure: (proxy) => {
          // Handle proxy errors
          proxy.on('error', (err, _req, res) => {
            console.error('proxy error', err);
            // Prevent connection hanging
            if (!res.headersSent) {
              res.writeHead(500, {
                'Content-Type': 'text/plain',
              });
            }
            res.end('Proxy error: ' + err.message);
          });

          // Handle WebSocket specific errors
          proxy.on('proxyReqWs', (_proxyReq, _req, socket) => {
            socket.on('error', (err) => {
              console.error('WebSocket proxy error:', err);
              socket.end();
            });
          });

          // Cleanup on WebSocket close
          proxy.on('close', () => {
            console.log('Proxy connection closed');
          });

          // Debug logging for development
          if (process.env.NODE_ENV === 'development') {
            proxy.on('proxyReq', (_proxyReq, req) => {
              console.debug(`[DEV] Proxying ${req.method} ${req.url}`);
            });
            
            proxy.on('proxyRes', (proxyRes, req) => {
              console.debug(`[DEV] Received ${proxyRes.statusCode} for ${req.url}`);
            });
          }
        },
      }
    },
    hmr: {
      // Properly handle HMR connections
      protocol: 'ws',
      host: 'localhost',
      port: 3000,
      clientPort: 3000,
      timeout: 5000,
      overlay: true,
    },
  },
})
