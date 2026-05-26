import { defineConfig } from 'astro/config'
import tailwindcss from '@tailwindcss/vite'
import singleFile from './integrations/single-file.mjs'

// https://astro.build/config
export default defineConfig({
  integrations: [singleFile()],
  vite: {
    plugins: [tailwindcss()]
  }
})
