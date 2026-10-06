import { defineConfig } from 'vite';

export default defineConfig({
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true
      }
    }
  },
  build: {
    outDir: 'dist',
    target: 'esnext',
    minify: 'esbuild', // P1-12: Enable minification
    cssMinify: true,
    modulePreload: { polyfill: false },
    cssCodeSplit: true,
    reportCompressedSize: true,
    sourcemap: false,
    emptyOutDir: true, // P1-13: Clean up old build artifacts
    assetsInlineLimit: 4096, // default value
    chunkSizeWarningLimit: 800,
    rollupOptions: {
      treeshake: true, // P1-12: Enable tree-shaking
      output: {
        manualChunks(id) {
          if (id.includes('node_modules/chart.js') || id.includes('node_modules/chartjs')) {
            return 'chartjs';
          }
        }
      }
    }
  },
  esbuild: {
    legalComments: 'none',
    target: 'esnext'
  }
});

