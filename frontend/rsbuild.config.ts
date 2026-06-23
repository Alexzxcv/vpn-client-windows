import { defineConfig } from '@rsbuild/core';
import { pluginReact } from '@rsbuild/plugin-react';

export default defineConfig({
  plugins: [pluginReact()],
  source: {
    entry: {
      index: './src/main.tsx',
    },
  },
  html: {
    title: 'SAPN VPN',
    favicon: './src/assets/favicon.svg',
  },
  output: {
    // Файлы отдаются с file-сервера ядра как статика (same-origin),
    // поэтому пути к ассетам должны быть относительными.
    assetPrefix: './',
    distPath: {
      root: 'dist',
    },
  },
});
