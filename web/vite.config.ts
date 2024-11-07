import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'
import tailwindcss from 'tailwindcss'
import autoprefixer from 'autoprefixer'

export default defineConfig(({ mode }) => ({
  base: "/",
  build: {
    outDir: 'dist',
    manifest: true,
    sourcemap: true,
    assetsDir: 'assets',
    modulePreload: false,
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['react', 'react-dom']
        },
        entryFileNames: 'assets/[name]-[hash].js',
        chunkFileNames: 'assets/[name]-[hash].js',
        assetFileNames: ({name}) => {
          // Keep the original file name without hash for PWA assets
          if (name && /\.(png|jpg|jpeg|svg|ico)$/.test(name)) {
            return name
          }
          return 'assets/[name]-[hash].[ext]'
        }
      }
    }
  },
  optimizeDeps: {
    include: ['react', 'react-dom'],
    exclude: ['virtual:pwa-register']
  },
  css: {
    modules: {
      localsConvention: 'camelCase'
    },
    postcss: {
      plugins: [tailwindcss, autoprefixer]
    }
  },
  plugins: [
    react({
      jsxRuntime: 'automatic'
    }),
    VitePWA({
      registerType: 'autoUpdate',
      injectRegister: 'auto',
      selfDestroying: false,
      filename: 'sw.js',
      manifestFilename: 'manifest.json',
      strategies: 'generateSW',
      includeAssets: [
        'favicon.ico',
        'apple-touch-icon.png',
        'masked-icon.svg',
        'pwa-192x192.png',
        'pwa-512x512.png',
        'apple-touch-icon-iphone-60x60.png',
        'apple-touch-icon-ipad-76x76.png',
        'apple-touch-icon-iphone-retina-120x120.png',
        'apple-touch-icon-ipad-retina-152x152.png',
        'logo.svg'
      ],
      manifest: {
        name: 'dashbrr',
        short_name: 'dashbrr',
        description: 'A dashboard for monitoring autobrr and related services',
        theme_color: '#18181B',
        background_color: '#18181B',
        display: 'standalone',
        icons: [
          {
            src: '/pwa-192x192.png',
            sizes: '192x192',
            type: 'image/png'
          },
          {
            src: '/pwa-512x512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'any maskable'
          },
          {
            src: '/apple-touch-icon-iphone-60x60.png',
            sizes: '60x60',
            type: 'image/png'
          },
          {
            src: '/apple-touch-icon-ipad-76x76.png',
            sizes: '76x76',
            type: 'image/png'
          },
          {
            src: '/apple-touch-icon-iphone-retina-120x120.png',
            sizes: '120x120',
            type: 'image/png'
          },
          {
            src: '/apple-touch-icon-ipad-retina-152x152.png',
            sizes: '152x152',
            type: 'image/png'
          }
        ],
        start_url: '/',
        scope: '/'
      },
      workbox: mode === 'production' ? {
        globDirectory: 'dist',
        globPatterns: [
          '**/*.{js,css,html,ico,png,svg}'
        ],
        navigateFallback: '/index.html',
        navigateFallbackDenylist: [/^\/api\//],
        runtimeCaching: [
          {
            urlPattern: /^https:\/\/fonts\.googleapis\.com\/.*/i,
            handler: 'CacheFirst',
            options: {
              cacheName: 'google-fonts-cache',
              expiration: {
                maxEntries: 10,
                maxAgeSeconds: 60 * 60 * 24 * 365 // 1 year
              },
              cacheableResponse: {
                statuses: [0, 200]
              }
            }
          },
          {
            urlPattern: /^(?!.*api).*$/,
            handler: 'NetworkFirst',
            options: {
              cacheName: 'app-shell',
              expiration: {
                maxEntries: 50,
                maxAgeSeconds: 60 * 60 * 24 // 24 hours
              },
              cacheableResponse: {
                statuses: [0, 200]
              }
            }
          }
        ],
        cleanupOutdatedCaches: true,
        skipWaiting: true,
        clientsClaim: true,
        sourcemap: true
      } : undefined,
      devOptions: {
        enabled: true,
        type: 'module',
        navigateFallback: '/index.html',
        suppressWarnings: true
      }
    })
  ],
  server: {
    port: 3000,
    hmr: {
      overlay: true,
      protocol: 'ws'
    },
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
        ws: true
      }
    }
  },
  preview: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
        ws: true
      }
    }
  }
}))
