import { readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { minify } from 'html-minifier-terser'

function collectFiles(folder) {
  return readdirSync(folder).flatMap((entry) => {
    const file = path.join(folder, entry)
    return statSync(file).isDirectory() ? collectFiles(file) : [file]
  })
}

function removeEmptyDirs(folder) {
  for (const entry of readdirSync(folder)) {
    const file = path.join(folder, entry)
    if (statSync(file).isDirectory()) {
      removeEmptyDirs(file)
      if (readdirSync(file).length === 0) {
        rmSync(file, { recursive: true })
      }
    }
  }
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export default function singleFile() {
  return {
    name: 'gol-single-file',
    hooks: {
      'astro:build:done': async ({ dir }) => {
        const folder = fileURLToPath(dir)
        const files = collectFiles(folder)
        const htmlFiles = files.filter((file) => file.endsWith('.html'))
        const cssAssets = files.filter((file) => file.endsWith('.css'))
        const jsAssets = files.filter((file) => file.endsWith('.js'))

        for (const htmlPath of htmlFiles) {
          let contents = readFileSync(htmlPath, 'utf8')
          for (const cssPath of cssAssets) {
            const cssFilename = escapeRegExp(path.basename(cssPath))
            const cssStyles = readFileSync(cssPath, 'utf8')
            const cssLink = new RegExp(`<link[^>]*? href="[^"]*${cssFilename}"[^>]*?>`)
            contents = contents.replace(cssLink, () => `<style type="text/css">\n${cssStyles}\n</style>`)
          }
          contents = await minify(contents, {
            collapseWhitespace: true,
            keepClosingSlash: true,
            removeComments: true,
            removeRedundantAttributes: true,
            removeScriptTypeAttributes: true,
            removeStyleLinkTypeAttributes: true,
            useShortDoctype: true,
            minifyCSS: true
          })
          for (const jsPath of jsAssets) {
            const jsFilename = escapeRegExp(path.basename(jsPath))
            const jsSource = readFileSync(jsPath, 'utf8').replace(/<\/script/gi, '<\\/script')
            const jsScript = new RegExp(`<script[^>]*?src="[^"]*${jsFilename}"[^>]*?></script>`)
            contents = contents.replace(jsScript, () => `<script type="module">\n${jsSource}\n</script>`)
          }
          writeFileSync(htmlPath, contents)
        }

        for (const cssPath of cssAssets) {
          rmSync(cssPath)
        }
        for (const jsPath of jsAssets) {
          rmSync(jsPath)
        }
        removeEmptyDirs(folder)
      }
    }
  }
}
