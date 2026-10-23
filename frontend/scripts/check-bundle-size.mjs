import { gzipSync } from 'node:zlib'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import path from 'node:path'

const dist = path.resolve(import.meta.dirname, '../../internal/web/dist')
const assets = path.join(dist, 'assets')
const limits = {
  entryGzip: 220 * 1024,
  chunkGzip: 650 * 1024,
  cssGzip: 100 * 1024,
}

const index = readFileSync(path.join(dist, 'index.html'), 'utf8')
const initialAssets = new Set(
  [...index.matchAll(/(?:src|href)="\/app\/assets\/([^"]+)"/g)].map((match) => match[1])
)

let initialGzip = 0
let largestChunk = { name: '', size: 0 }
let largestCSS = { name: '', size: 0 }

for (const name of readdirSync(assets)) {
  const full = path.join(assets, name)
  if (!statSync(full).isFile()) continue
  const gzip = gzipSync(readFileSync(full), { level: 9 }).byteLength
  if (name.endsWith('.js')) {
    if (gzip > largestChunk.size) largestChunk = { name, size: gzip }
    if (initialAssets.has(name)) initialGzip += gzip
  } else if (name.endsWith('.css') && gzip > largestCSS.size) {
    largestCSS = { name, size: gzip }
  }
}

const kib = (bytes) => `${(bytes / 1024).toFixed(1)} KiB gzip`
const failures = []
if (initialGzip > limits.entryGzip) {
  failures.push(`initial JS ${kib(initialGzip)} exceeds ${kib(limits.entryGzip)}`)
}
if (largestChunk.size > limits.chunkGzip) {
  failures.push(
    `chunk ${largestChunk.name} ${kib(largestChunk.size)} exceeds ${kib(limits.chunkGzip)}`
  )
}
if (largestCSS.size > limits.cssGzip) {
  failures.push(
    `CSS ${largestCSS.name} ${kib(largestCSS.size)} exceeds ${kib(limits.cssGzip)}`
  )
}
if (failures.length) {
  console.error(failures.join('\n'))
  process.exit(1)
}

console.log(
  `bundle budget ok: initial ${kib(initialGzip)}, largest chunk ${kib(largestChunk.size)}, CSS ${kib(largestCSS.size)}`
)
