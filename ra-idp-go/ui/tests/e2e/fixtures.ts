import { createHash } from 'node:crypto'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createServer, type Server, type Socket } from 'node:net'
import { spawn, spawnSync, type Subprocess } from 'bun'

const here = dirname(fileURLToPath(import.meta.url))
const uiDir = resolve(here, '../..')
const goDir = resolve(here, '../../..')

export const uiOrigin = 'http://localhost:5173'
export const apiHealth = 'http://localhost:8081/health'
export const callbackPort = 3000

export const demo = {
  clientId: 'demo-client',
  username: 'alice',
  password: 'demo-password-1234',
  email: 'alice@example.com',
  redirectUri: 'http://localhost:3000/callback',
  scope: 'openid profile email offline_access',
}

// /authorize は PKCE 必須 (routes_e2e_test.go と同条件)。本スモークは
// 認可コードの token 交換まではせず callback URL の code / iss を見るだけなので、
// 固定 verifier の S256 challenge を載せれば十分。
const verifier = 'ra-idp-e2e-pkce-verifier-0123456789abcdefghij'
const codeChallenge = createHash('sha256').update(verifier).digest('base64url')

export function authorizePath(state: string): string {
  const params = new URLSearchParams({
    client_id: demo.clientId,
    redirect_uri: demo.redirectUri,
    response_type: 'code',
    scope: demo.scope,
    state,
    code_challenge: codeChallenge,
    code_challenge_method: 'S256',
  })
  return `/authorize?${params.toString()}`
}

let goServer: Subprocess | undefined
let viteServer: Subprocess | undefined
let goBinary: string | undefined
let callback: ReturnType<typeof Bun.serve> | undefined
let mailSink: Server | undefined
let mailSinkPort = 0
const sentEmails: CapturedEmail[] = []

type CapturedEmail = {
  to: string
  raw: string
  text: string
}

export async function isUp(url: string): Promise<boolean> {
  try {
    return (await fetch(url)).ok
  } catch {
    return false
  }
}

export async function waitForUp(url: string, timeoutMs = 120_000): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (await isUp(url)) {
      return
    }
    await Bun.sleep(500)
  }
  throw new Error(`timeout waiting for ${url}`)
}

export async function startE2EEnvironment(): Promise<void> {
  if (!(await isUp(`http://localhost:${callbackPort}/health`))) {
    try {
      callback = Bun.serve({ port: callbackPort, fetch: () => new Response('received') })
    } catch {
      // 既存プロセスが redirect_uri の callback port を使っている場合は再利用する。
    }
  }

  if (!(await isUp(apiHealth))) {
    await startMailSink()
    goBinary = join(tmpdir(), `ra-idp-go-e2e-${process.pid}`)
    const build = spawnSync(['go', 'build', '-o', goBinary, './cmd/ra-idp-go'], { cwd: goDir })
    if (build.exitCode !== 0) {
      throw new Error(`go build failed: ${build.stderr?.toString() ?? ''}`)
    }
    goServer = spawn([goBinary], {
      cwd: goDir,
      env: {
        ...process.env,
        ADDR: ':8081',
        ISSUER: uiOrigin,
        PERSISTENCE: 'memory',
        EMAIL_SENDER: 'smtp',
        SMTP_HOST: '127.0.0.1',
        SMTP_PORT: String(mailSinkPort),
        SMTP_FROM: 'noreply@ra-idp.test',
        SMTP_TLS: 'none',
        SMTP_TIMEOUT_SECONDS: '2',
      },
      stdout: 'ignore',
      stderr: 'ignore',
    })
  }

  if (!(await isUp(uiOrigin))) {
    viteServer = spawn(['bun', 'run', 'dev'], { cwd: uiDir, stdout: 'ignore', stderr: 'ignore' })
  }

  await waitForUp(apiHealth)
  await waitForUp(uiOrigin)
}

export function stopE2EEnvironment(): void {
  goServer?.kill()
  viteServer?.kill()
  callback?.stop(true)
  mailSink?.close()
  goServer = undefined
  viteServer = undefined
  callback = undefined
  mailSink = undefined
  mailSinkPort = 0
  sentEmails.length = 0
}

async function startMailSink(): Promise<void> {
  if (mailSink) return
  mailSink = createServer((socket) => handleSMTPConnection(socket))
  await new Promise<void>((resolve, reject) => {
    mailSink?.once('error', reject)
    mailSink?.listen(0, '127.0.0.1', () => {
      mailSink?.off('error', reject)
      const addr = mailSink?.address()
      if (addr && typeof addr === 'object') {
        mailSinkPort = addr.port
        resolve()
        return
      }
      reject(new Error('mail sink did not expose a TCP port'))
    })
  })
}

function handleSMTPConnection(socket: Socket): void {
  let buffer = ''
  let inData = false
  let recipient = ''
  let dataLines: string[] = []

  const write = (line: string) => socket.write(`${line}\r\n`)
  write('220 ra-idp-e2e-smtp ready')

  socket.on('data', (chunk) => {
    buffer += chunk.toString('utf8')
    while (buffer.includes('\n')) {
      const index = buffer.indexOf('\n')
      const rawLine = buffer.slice(0, index)
      buffer = buffer.slice(index + 1)
      const line = rawLine.replace(/\r$/, '')

      if (inData) {
        if (line === '.') {
          const raw = dataLines.join('\r\n')
          sentEmails.push({ to: recipient, raw, text: decodeSMTPText(raw) })
          dataLines = []
          inData = false
          write('250 ok')
          continue
        }
        dataLines.push(line)
        continue
      }

      const upper = line.toUpperCase()
      if (upper.startsWith('EHLO') || upper.startsWith('HELO')) {
        write('250-ra-idp-e2e-smtp')
        write('250 HELP')
      } else if (upper.startsWith('MAIL FROM')) {
        write('250 ok')
      } else if (upper.startsWith('RCPT TO')) {
        recipient = extractSMTPAddress(line)
        write('250 ok')
      } else if (upper === 'DATA') {
        inData = true
        write('354 end data with <CR><LF>.<CR><LF>')
      } else if (upper === 'QUIT') {
        write('221 bye')
        socket.end()
      } else if (upper === 'NOOP') {
        write('250 ok')
      } else {
        write('250 ok')
      }
    }
  })
}

function extractSMTPAddress(line: string): string {
  const match = line.match(/<([^>]+)>/)
  return match?.[1] ?? ''
}

function decodeSMTPText(raw: string): string {
  const chunks = raw
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => /^[A-Za-z0-9+/=]+$/.test(line))
  if (chunks.length === 0) return raw
  try {
    return Buffer.from(chunks.join(''), 'base64').toString('utf8')
  } catch {
    return raw
  }
}

export async function waitForEmailURL(
  to: string,
  path: string,
  timeoutMs = 10_000,
): Promise<string> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const found = [...sentEmails].reverse().find((email) => email.to === to && email.text.includes(path))
    if (found) {
      const match = found.text.match(/https?:\/\/\S+/)
      if (match) return match[0]
    }
    await Bun.sleep(150)
  }
  throw new Error(`timeout waiting for email url: to=${to} path=${path}`)
}

export function metaPage(view: Bun.WebView): Promise<unknown> {
  return view.evaluate(
    'document.querySelector(\'meta[name="ra-idp:page"]\')?.getAttribute("content") ?? null',
  )
}

export async function waitForPage(
  view: Bun.WebView,
  kind: string,
  timeoutMs = 15_000,
): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    try {
      if ((await metaPage(view)) === kind) {
        return
      }
    } catch {
      // 遷移中は evaluate が失敗しうる。リトライする。
    }
    await Bun.sleep(150)
  }
  throw new Error(`timeout waiting for page kind=${kind}, last url=${view.url}`)
}

export async function waitForUrl(
  view: Bun.WebView,
  pattern: RegExp,
  timeoutMs = 15_000,
): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (pattern.test(view.url)) {
      return
    }
    await Bun.sleep(150)
  }
  throw new Error(`timeout waiting for url ${pattern}, last=${view.url}`)
}

export async function waitForLocationPath(
  view: Bun.WebView,
  expected: string,
  timeoutMs = 15_000,
): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if ((await view.evaluate('window.location.pathname')) === expected) {
      return
    }
    await Bun.sleep(150)
  }
  throw new Error(`timeout waiting for location.pathname=${expected}`)
}

export async function clickButtonByText(view: Bun.WebView, text: string): Promise<void> {
  const clicked = await view.evaluate(`(() => {
    const target = [...document.querySelectorAll('button')]
      .find((b) => (b.textContent ?? '').includes(${JSON.stringify(text)}))
    if (!target) return false
    target.click()
    return true
  })()`)
  if (clicked !== true) {
    throw new Error(`button not found: ${text}`)
  }
}

export async function clickLinkByText(view: Bun.WebView, text: string): Promise<void> {
  const clicked = await view.evaluate(`(() => {
    const target = [...document.querySelectorAll('a')]
      .find((link) => (link.textContent ?? '').includes(${JSON.stringify(text)}))
    if (!(target instanceof HTMLElement)) return false
    target.click()
    return true
  })()`)
  if (clicked !== true) {
    throw new Error(`link not found: ${text}`)
  }
}

export async function hasText(view: Bun.WebView, text: string): Promise<boolean> {
  return view.evaluate(
    `document.body.textContent?.includes(${JSON.stringify(text)}) ?? false`,
  ) as Promise<boolean>
}

export async function waitForText(
  view: Bun.WebView,
  text: string,
  timeoutMs = 10_000,
): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    if (await hasText(view, text)) {
      return
    }
    await Bun.sleep(150)
  }
  throw new Error(`timeout waiting for text: ${text}`)
}

export async function setInputValue(
  view: Bun.WebView,
  selector: string,
  value: string,
): Promise<void> {
  const changed = await view.evaluate(`(() => {
    const input = document.querySelector(${JSON.stringify(selector)})
    if (!(input instanceof HTMLInputElement || input instanceof HTMLTextAreaElement)) return false
    input.focus()
    const prototype = input instanceof HTMLInputElement
      ? HTMLInputElement.prototype
      : HTMLTextAreaElement.prototype
    const descriptor = Object.getOwnPropertyDescriptor(prototype, 'value')
    descriptor?.set?.call(input, ${JSON.stringify(value)})
    input.dispatchEvent(new InputEvent('input', {
      bubbles: true,
      inputType: 'insertReplacementText',
      data: ${JSON.stringify(value)},
    }))
    input.dispatchEvent(new Event('change', { bubbles: true }))
    return true
  })()`)
  if (changed !== true) {
    throw new Error(`input not found: ${selector}`)
  }
}

export async function setSelectValue(
  view: Bun.WebView,
  selector: string,
  value: string,
): Promise<void> {
  const changed = await view.evaluate(`(() => {
    const select = document.querySelector(${JSON.stringify(selector)})
    if (!(select instanceof HTMLSelectElement)) return false
    select.value = ${JSON.stringify(value)}
    select.dispatchEvent(new Event('input', { bubbles: true }))
    select.dispatchEvent(new Event('change', { bubbles: true }))
    return true
  })()`)
  if (changed !== true) {
    throw new Error(`select not found: ${selector}`)
  }
}

export async function selectDropdownOption(
  view: Bun.WebView,
  triggerText: string,
  optionText: string,
): Promise<void> {
  const opened = await view.evaluate(`(() => {
    const target = [...document.querySelectorAll('button')]
      .find((button) => (button.textContent ?? '').includes(${JSON.stringify(triggerText)}))
    if (!(target instanceof HTMLElement)) return false
    target.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0 }))
    target.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }))
    target.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, button: 0 }))
    target.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, button: 0 }))
    target.click()
    return true
  })()`)
  if (opened !== true) {
    throw new Error(`dropdown trigger not found: ${triggerText}`)
  }

  const deadline = Date.now() + 10_000
  while (Date.now() < deadline) {
    const selected = await view.evaluate(`(() => {
      const target = [...document.querySelectorAll('[role="menuitem"]')]
        .find((item) => (item.textContent ?? '').includes(${JSON.stringify(optionText)}))
      if (!(target instanceof HTMLElement)) return false
      target.click()
      return true
    })()`)
    if (selected === true) return
    await Bun.sleep(150)
  }
  const available = await view.evaluate(`(() => [...document.querySelectorAll('[role="menuitem"]')]
    .map((item) => item.textContent?.trim() ?? '')
    .filter(Boolean)
    .join(' | '))()`)
  throw new Error(`dropdown option not found: ${optionText}; available=${available}`)
}

export async function clickMenuItemByText(view: Bun.WebView, text: string): Promise<void> {
  const deadline = Date.now() + 10_000
  while (Date.now() < deadline) {
    const clicked = await view.evaluate(`(() => {
      const target = [...document.querySelectorAll('[role="menuitem"]')]
        .find((item) => (item.textContent ?? '').includes(${JSON.stringify(text)}))
      if (!(target instanceof HTMLElement)) return false
      target.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0 }))
      target.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }))
      target.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, button: 0 }))
      target.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, button: 0 }))
      target.click()
      return true
    })()`)
    if (clicked === true) return
    await Bun.sleep(150)
  }
  throw new Error(`menu item not found: ${text}`)
}

export async function setCheckboxValue(
  view: Bun.WebView,
  selector: string,
  checked: boolean,
): Promise<void> {
  const changed = await view.evaluate(`(() => {
    const input = document.querySelector(${JSON.stringify(selector)})
    if (!(input instanceof HTMLInputElement) || input.type !== 'checkbox') return false
    input.checked = ${checked ? 'true' : 'false'}
    input.dispatchEvent(new Event('input', { bubbles: true }))
    input.dispatchEvent(new Event('change', { bubbles: true }))
    return true
  })()`)
  if (changed !== true) {
    throw new Error(`checkbox not found: ${selector}`)
  }
}

export async function clickElementByAriaLabel(view: Bun.WebView, label: string): Promise<void> {
  const clicked = await view.evaluate(`(() => {
    const target = document.querySelector(${JSON.stringify(`[aria-label="${label}"]`)})
    if (!(target instanceof HTMLElement)) return false
    target.click()
    return true
  })()`)
  if (clicked !== true) {
    throw new Error(`element not found by aria-label: ${label}`)
  }
}

export async function clickNavLinkByText(
  view: Bun.WebView,
  navLabel: string,
  text: string,
): Promise<void> {
  const clicked = await view.evaluate(`(() => {
    const nav = document.querySelector(${JSON.stringify(`nav[aria-label="${navLabel}"]`)})
    if (!nav) return false
    const target = [...nav.querySelectorAll('a')]
      .find((a) => (a.textContent ?? '').trim() === ${JSON.stringify(text)})
    if (!target) return false
    target.click()
    return true
  })()`)
  if (clicked !== true) {
    throw new Error(`nav link not found: ${navLabel} / ${text}`)
  }
}

export async function loginFromCurrentPage(view: Bun.WebView): Promise<void> {
  await waitForPage(view, 'login')
  await view.click('input[name="username"]')
  await view.type(demo.username)
  await view.click('input[name="password"]')
  await view.type(demo.password)
  await view.click('button[type="submit"]')
}

export async function navigateAndLogin(
  view: Bun.WebView,
  path: string,
  expectedPage: string,
): Promise<void> {
  await view.navigate(`${uiOrigin}${path}`)
  await loginFromCurrentPage(view)
  await waitForPage(view, expectedPage, 30_000)
}
