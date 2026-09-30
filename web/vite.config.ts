import { defineConfig } from 'vite';
import { resolve } from 'path';

export default defineConfig({
  root: '.',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        main: resolve(__dirname, 'index.html'),
        setup: resolve(__dirname, 'setup.html'),
      },
    },
    // Target modern Chromium on Raspberry Pi OS (Chromium 100+)
    target: 'es2022',
    minify: 'esbuild',
  },
});
