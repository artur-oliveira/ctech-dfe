/**
 * Captura as telas reais do app em WebP, para o guia (/guide) e a landing page.
 *
 * Roda contra o dev server com o mock API ligado — as fixtures de
 * `src/lib/mock/` são o que aparece nas imagens, então nenhum dado real de
 * cliente entra no repositório:
 *
 *   NEXT_PUBLIC_MOCK_API=true npm run dev
 *   npm run screens:capture
 *
 * Filtro opcional por slug: `npm run screens:capture -- nfe-` captura só as
 * telas cujo slug começa com `nfe-`.
 *
 * Cada nova feature entra aqui como uma entrada de CAPTURES e como uma seção
 * no guia — ver `../DOCS.md §5`, "Guia do produto (/guide) e capturas de tela".
 */

import {spawn} from 'node:child_process'
import {mkdir, writeFile} from 'node:fs/promises'
import {resolve} from 'node:path'
// As chaves saem das próprias fixtures: se uma mudar, a captura acompanha em vez
// de apontar para um documento que não existe mais.
import {mdfesFixture, nfcesFixture, nfesFixture, nfsesFixture} from '../src/lib/mock/fixtures.ts'

const ORIGIN = process.env.CAPTURE_ORIGIN || 'http://127.0.0.1:3000'
const OUTPUT_DIR = resolve('public/guide')
const DEVTOOLS_PORT = 9224

const DESKTOP = {width: 1280, height: 800, deviceScaleFactor: 1, mobile: false}
const MOBILE = {width: 390, height: 844, deviceScaleFactor: 2, mobile: true}

const NFE_KEY = nfesFixture[0].sk
const NFCE_KEY = nfcesFixture[0].sk
const MDFE_KEY = mdfesFixture[0].sk
const NFSE_ID = nfsesFixture[0].sk

/**
 * Passos de preparação. Cada um é `{tipo: alvo}`:
 *   click     — clica no primeiro botão/link cujo texto contém o alvo
 *   clickAt   — clica no seletor CSS
 *   waitText  — espera texto aparecer em qualquer lugar da página
 *   waitAt    — espera o seletor CSS existir
 *   scrollAt  — rola até o seletor CSS
 *   pause     — espera N ms (último recurso; prefira waitText/waitAt)
 */
const CAPTURES = [
  // ── Visão geral ──────────────────────────────────────────────────────────
  {slug: 'dashboard', route: '/dashboard', waitText: 'Acesso rápido'},
  // A raiz do onboarding redireciona para o passo pendente, que varia; a última
  // etapa é o alvo estável para a imagem do guia.
  {slug: 'onboarding', route: '/onboarding/pronto', waitText: 'Documentos habilitados'},

  // ── NF-e ─────────────────────────────────────────────────────────────────
  {slug: 'nfe-list', route: '/nfe?tab=emitidas', waitText: 'Cliente Exemplo LTDA'},
  {
    slug: 'nfe-emit-receiver',
    route: '/nfe/emit',
    waitText: 'Destinatário',
  },
  {
    slug: 'nfe-emit-products',
    route: '/nfe/emit',
    steps: [
      {waitText: 'Cliente Exemplo LTDA'},
      {click: 'Cliente Exemplo LTDA'},
      {click: 'Próximo'},
      {waitText: 'Adicionar produto'},
      {click: 'Adicionar produto'},
      {waitText: 'Notebook 15 polegadas'},
      {click: 'Notebook 15 polegadas'},
    ],
  },
  {
    slug: 'nfe-emit-review',
    route: '/nfe/emit',
    steps: [
      {waitText: 'Cliente Exemplo LTDA'},
      {click: 'Cliente Exemplo LTDA'},
      {click: 'Próximo'},
      {waitText: 'Adicionar produto'},
      {click: 'Adicionar produto'},
      {waitText: 'Notebook 15 polegadas'},
      {click: 'Notebook 15 polegadas'},
      {click: 'Próximo'},
      {waitText: 'Condição de pagamento'},
      {click: 'Próximo'},
      {waitText: 'Emitir NF-e'},
    ],
  },
  {slug: 'nfe-detail', route: `/nfe/detail?key=${NFE_KEY}`, waitText: 'Notebook 15 polegadas'},
  {
    slug: 'nfe-events',
    route: `/nfe/detail?key=${NFE_KEY}`,
    steps: [
      {waitText: 'Carta de Correção'},
      {scrollAt: '[aria-label="Eventos do documento"]'},
    ],
  },
  {
    slug: 'nfe-cancel',
    route: `/nfe/detail?key=${NFE_KEY}`,
    steps: [{waitText: 'Cancelar'}, {click: 'Cancelar'}, {waitAt: '[role="dialog"]'}],
  },
  {slug: 'nfe-distribution', route: '/nfe?tab=distribuicao', waitText: 'NSU'},

  // ── NFC-e ────────────────────────────────────────────────────────────────
  {slug: 'nfce-list', route: '/nfce', waitText: 'Cliente Exemplo LTDA'},
  {slug: 'nfce-emit', route: '/nfce/emit', waitText: 'Escaneie os itens'},
  {slug: 'nfce-detail', route: `/nfce/detail?key=${NFCE_KEY}`, waitText: 'Mouse óptico'},

  // ── CT-e / MDF-e ─────────────────────────────────────────────────────────
  {slug: 'cte-distribution', route: '/cte', waitText: 'CT-e'},
  {slug: 'mdfe-list', route: '/mdfe', waitText: 'Detalhes'},
  {slug: 'mdfe-emit', route: '/mdfe/emit', waitText: 'tipo de transporte'},
  {slug: 'mdfe-detail', route: `/mdfe/detail?key=${MDFE_KEY}`, waitText: 'Encerrar'},

  // ── NFS-e ────────────────────────────────────────────────────────────────
  {slug: 'nfse-list', route: '/nfse', waitText: 'Cliente Exemplo LTDA'},
  {slug: 'nfse-emit', route: '/nfse/emit', waitText: 'Prévia da DPS'},
  {slug: 'nfse-detail', route: `/nfse/detail?id=${NFSE_ID}`, waitText: 'Empresa Mock LTDA'},

  // ── Cadastros e configuração ─────────────────────────────────────────────
  {slug: 'products', route: '/products', waitText: 'Notebook 15 polegadas'},
  {slug: 'persons', route: '/persons', waitText: 'Cliente Exemplo LTDA'},
  {slug: 'tax-profiles', route: '/tax-profiles', waitText: 'Simples Nacional'},
  {slug: 'vehicles', route: '/vehicles', waitText: 'ABC1D23'},
  {slug: 'fiscal-config', route: '/fiscal-config', waitText: 'Ambiente ativo'},
  {slug: 'certificates', route: '/certificates', waitText: 'EMPRESA MOCK'},

  // ── Conta ────────────────────────────────────────────────────────────────
  {slug: 'subscription', route: '/assinatura', waitText: 'Assinatura'},
  {slug: 'audit-logs', route: '/audit-logs', waitText: 'Mock User'},

  // ── Estados ──────────────────────────────────────────────────────────────
  {
    slug: 'nfe-rejection',
    route: '/nfe?tab=emitidas',
    steps: [{waitText: 'Rejeitada'}, {click: 'Rejeitada'}, {waitAt: '[role="dialog"]'}],
  },
  {slug: 'empty-state', route: '/nfe?tab=transportadas', waitText: 'Transportadas'},

  // ── Mobile ───────────────────────────────────────────────────────────────
  {slug: 'mobile-dashboard', route: '/dashboard', viewport: MOBILE, waitText: 'Acesso rápido'},
  {slug: 'mobile-nfe-list', route: '/nfe', viewport: MOBILE, waitText: 'Nota Fiscal Eletrônica'},
]

const delay = (ms) => new Promise((r) => setTimeout(r, ms))

async function browserSocket() {
  for (let attempt = 0; attempt < 60; attempt += 1) {
    try {
      const response = await fetch(`http://127.0.0.1:${DEVTOOLS_PORT}/json/version`)
      if (response.ok) return (await response.json()).webSocketDebuggerUrl
    } catch {
      // Chrome ainda subindo.
    }
    await delay(150)
  }
  throw new Error('Chrome DevTools não ficou pronto.')
}

function cdp(socketUrl) {
  const socket = new WebSocket(socketUrl)
  let nextId = 0
  const pending = new Map()
  socket.onmessage = (event) => {
    const message = JSON.parse(event.data)
    const request = pending.get(message.id)
    if (!request) return
    pending.delete(message.id)
    if (message.error) request.reject(new Error(message.error.message))
    else request.resolve(message.result)
  }
  const ready = new Promise((res, rej) => {
    socket.onopen = res
    socket.onerror = rej
  })
  return {
    async send(method, params = {}, sessionId) {
      await ready
      const id = ++nextId
      socket.send(JSON.stringify({id, method, params, ...(sessionId ? {sessionId} : {})}))
      return new Promise((res, rej) => pending.set(id, {resolve: res, reject: rej}))
    },
    close: () => socket.close(),
  }
}

async function evaluate(send, expression) {
  const result = await send('Runtime.evaluate', {expression, awaitPromise: true, returnByValue: true})
  if (result.exceptionDetails) throw new Error(result.exceptionDetails.text || 'Falha ao avaliar no browser.')
  return result.result?.value
}

/** Texto como literal JS — as legendas têm acento, aspas e apóstrofos. */
const lit = (value) => JSON.stringify(value)

// Interativos primeiro: `querySelectorAll` devolve em ordem de documento, e a
// célula da tabela vem antes do botão dentro dela — clicar na célula não faz nada.
const CLICKABLE = 'button, a, [role="button"], [role="option"]'
const CLICKABLE_FALLBACK = 'li, td, tr'

async function waitFor(send, expression, label) {
  for (let attempt = 0; attempt < 150; attempt += 1) {
    if (await evaluate(send, expression)) return
    await delay(100)
  }
  throw new Error(`Timeout esperando ${label}.`)
}

async function runStep(send, step) {
  if (step.pause) return delay(step.pause)
  if (step.waitText) {
    // innerText respeita text-transform, então rótulos em caixa alta por CSS
    // não batem com o texto do código-fonte — compara em minúsculas.
    return waitFor(
      send,
      `document.body.innerText.toLowerCase().includes(${lit(step.waitText.toLowerCase())})`,
      `o texto "${step.waitText}"`,
    )
  }
  if (step.waitAt) {
    return waitFor(send, `Boolean(document.querySelector(${lit(step.waitAt)}))`, `o seletor ${step.waitAt}`)
  }
  if (step.scrollAt) {
    return evaluate(send, `document.querySelector(${lit(step.scrollAt)})?.scrollIntoView({block: 'start'})`)
  }
  if (step.clickAt) {
    await waitFor(send, `Boolean(document.querySelector(${lit(step.clickAt)}))`, `o seletor ${step.clickAt}`)
    return evaluate(send, `document.querySelector(${lit(step.clickAt)}).click()`)
  }
  if (step.click) {
    const match = `node => node.textContent?.toLowerCase().includes(${lit(step.click.toLowerCase())}) && !node.disabled`
    const finder = `(Array.from(document.querySelectorAll(${lit(CLICKABLE)})).find(${match})
      ?? Array.from(document.querySelectorAll(${lit(CLICKABLE_FALLBACK)})).find(${match}))`
    await waitFor(send, `Boolean(${finder})`, `o elemento clicável "${step.click}"`)
    await evaluate(send, `(${finder}).click()`)
    return delay(400)
  }
  throw new Error(`Passo desconhecido: ${JSON.stringify(step)}`)
}

/**
 * Tela quebrada vira imagem quebrada sem ninguém perceber — o guia publica o que
 * o script salvar. Falha alto quando a página não renderizou de verdade.
 */
const SANITY = `(() => {
  const text = document.body.innerText.trim();
  if (text.includes("This page couldn’t load") || text.includes('Application error')) return 'a página não carregou';
  if (text.length < 120) return \`a página tem só \${text.length} caracteres de texto\`;
  return null;
})()`

/**
 * Rascunhos de emissão (`pydfe_emit_draft_*`) sobrevivem entre navegações e a
 * captura seguinte abriria com a tarja "emissão não finalizada". Limpa antes.
 */
const CLEAR_DRAFTS = `(() => {
  Object.keys(localStorage)
    .filter(key => key.startsWith('pydfe_emit_draft'))
    .forEach(key => localStorage.removeItem(key));
  return true;
})()`

/** Congela animações e remove o cromo de dev antes do screenshot. */
const STABILIZE = `(() => {
  document.querySelectorAll('[data-mock-panel], nextjs-portal').forEach(node => node.remove());
  const style = document.createElement('style');
  style.textContent = '*, *::before, *::after { animation-duration: 1ms !important; animation-delay: 0s !important; transition: none !important; }';
  document.head.append(style);
  document.documentElement.style.scrollBehavior = 'auto';
  return true;
})()`

const filters = process.argv.slice(2).filter((arg) => !arg.startsWith('-'))
const captures = filters.length
  ? CAPTURES.filter((capture) => filters.some((filter) => capture.slug.startsWith(filter)))
  : CAPTURES

if (!captures.length) {
  process.stderr.write(`Nenhum slug casa com ${filters.join(', ')}.\n`)
  process.exit(1)
}

const chrome = spawn(
  process.env.CHROME_BIN || 'google-chrome',
  [
    '--headless=new',
    '--no-sandbox',
    '--disable-gpu',
    '--hide-scrollbars',
    '--disable-dev-shm-usage',
    `--remote-debugging-port=${DEVTOOLS_PORT}`,
    '--window-size=1920,1280',
    '--force-prefers-reduced-motion',
    'about:blank',
  ],
  {stdio: 'ignore'},
)

let failures = 0

try {
  await mkdir(OUTPUT_DIR, {recursive: true})
  const browser = cdp(await browserSocket())
  const {targetId} = await browser.send('Target.createTarget', {url: 'about:blank'})
  const {sessionId} = await browser.send('Target.attachToTarget', {targetId, flatten: true})
  const send = (method, params = {}) => browser.send(method, params, sessionId)
  await send('Page.enable')

  for (const capture of captures) {
    const output = resolve(OUTPUT_DIR, `${capture.slug}.webp`)
    try {
      await send('Emulation.setDeviceMetricsOverride', capture.viewport ?? DESKTOP)
      await send('Page.navigate', {url: `${ORIGIN}/dashboard`})
      await delay(600)
      await evaluate(send, CLEAR_DRAFTS)
      await send('Page.navigate', {url: `${ORIGIN}${capture.route}`})
      // O app monta a sessão do mock antes de qualquer query resolver.
      await delay(1500)
      const steps = capture.steps ?? (capture.waitText ? [{waitText: capture.waitText}] : [])
      for (const step of steps) await runStep(send, step)
      await delay(500)
      const problem = await evaluate(send, SANITY)
      if (problem) throw new Error(problem)
      await evaluate(send, STABILIZE)
      const {data} = await send('Page.captureScreenshot', {
        format: 'webp',
        quality: 92,
        fromSurface: true,
        captureBeyondViewport: false,
      })
      await writeFile(output, Buffer.from(data, 'base64'))
      process.stdout.write(`✓ ${capture.slug}.webp  ←  ${capture.route}\n`)
    } catch (error) {
      failures += 1
      process.stderr.write(`✗ ${capture.slug}  (${capture.route}): ${error.message}\n`)
    }
  }
  browser.close()
} finally {
  chrome.kill('SIGTERM')
}

if (failures) {
  process.stderr.write(`\n${failures} captura(s) falharam.\n`)
  process.exit(1)
}
