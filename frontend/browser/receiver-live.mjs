import {spawn, execFileSync} from 'node:child_process'
import {mkdtempSync, readFileSync, rmSync} from 'node:fs'
import {tmpdir} from 'node:os'
import {join, resolve} from 'node:path'
import {fileURLToPath} from 'node:url'
import {chromium} from 'playwright'

const root = resolve(fileURLToPath(new URL('../..', import.meta.url)))
const temp = mkdtempSync(join(tmpdir(), 'fairdrop-receiver-'))
const binary = join(temp, process.platform === 'win32' ? 'fixture.exe' : 'fixture')
let fixture
let fixtureExited
let browser
let fixtureOutput = ''
let fixtureErrors = ''

async function within(promise, milliseconds, label) {
    let timer
    try {
        return await Promise.race([
            promise,
            new Promise((_, reject) => { timer = setTimeout(() => reject(new Error(`${label} exceeded ${milliseconds}ms\n${fixtureOutput}\n${fixtureErrors}`)), milliseconds) }),
        ])
    } finally {
        clearTimeout(timer)
    }
}

async function checkPage({url, longUrl}) {
    browser = await chromium.launch({headless: true})
    const context = await browser.newContext({viewport: {width: 320, height: 700}, acceptDownloads: true})
    const page = await context.newPage()
    const response = await page.goto(url)
    if (response?.status() !== 200) throw new Error(`GET status = ${response?.status()}`)
    const headers = response.headers()
    if (headers['content-security-policy'] !== "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'") throw new Error('live CSP differs from receiver policy')
    if (headers['referrer-policy'] !== 'no-referrer' || headers['cache-control'] !== 'no-store') throw new Error('live privacy headers missing')
    if (await page.locator('h1').textContent() !== 'Ready to download') throw new Error('receiver heading missing')
    if (await page.locator('.name').textContent() !== 'quarterly report.pdf') throw new Error('staged display name missing')
    if (await page.locator('.detail').textContent() !== 'File · 12 bytes') throw new Error('staged file size missing')
    if (await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)) throw new Error('page scrolls horizontally at 320px')
    const button = page.getByRole('button', {name: 'Download'})
    if (await button.evaluate((element) => getComputedStyle(element).minHeight) !== '48px') throw new Error('Download target too small')
    await page.keyboard.press('Tab')
    if (!(await button.evaluate((element) => element === document.activeElement))) throw new Error('Download is not keyboard reachable')
    if (await button.evaluate((element) => getComputedStyle(element).outlineStyle) === 'none') throw new Error('Download has no keyboard focus outline')
    const longPage = await context.newPage()
    if ((await longPage.goto(longUrl))?.status() !== 200) throw new Error('long-name page failed')
    if (await longPage.locator('.name').textContent() !== '長'.repeat(240) + 'שלום<unsafe>') throw new Error('long Unicode name changed')
    if (await longPage.locator('img,script,link').count() !== 0) throw new Error('name injected an element or resource')
    if (await longPage.evaluate(() => document.documentElement.scrollWidth > innerWidth)) throw new Error('long name scrolls horizontally at 320px')
    await longPage.emulateMedia({colorScheme: 'dark'})
    if (await longPage.evaluate(() => getComputedStyle(document.documentElement).backgroundColor) !== 'rgb(22, 22, 24)') throw new Error('dark Quartz canvas missing')
    await longPage.emulateMedia({forcedColors: 'active'})
    await longPage.getByRole('button', {name: 'Download'}).focus()
    if (await longPage.getByRole('button', {name: 'Download'}).evaluate((element) => getComputedStyle(element).outlineStyle) === 'none') throw new Error('forced-colors focus outline missing')
    await longPage.emulateMedia({forcedColors: 'none'})
    await longPage.evaluate(() => { document.body.style.zoom = '2' })
    const zoomWidth = await longPage.evaluate(() => ({scroll: document.documentElement.scrollWidth, viewport: innerWidth}))
    if (zoomWidth.scroll > zoomWidth.viewport) throw new Error(`long name scrolls horizontally at 200% zoom: ${JSON.stringify(zoomWidth)}`)
    const requestSeen = page.waitForRequest((request) => request.url() === url && request.method() === 'POST')
    const downloadSeen = page.waitForEvent('download')
    await page.keyboard.press('Enter')
    await requestSeen
    const download = await downloadSeen
    if (download.suggestedFilename() !== 'quarterly report.pdf') throw new Error(`attachment filename = ${download.suggestedFilename()}`)
    const saved = await download.path()
    if (await download.failure() !== null || readFileSync(saved, 'utf8') !== 'hello world!') throw new Error('browser download bytes differ from 12-byte staged fixture')
    console.log('receiver live browser: real GET page, headers, 320px layout, keyboard POST and exact attachment bytes passed')
}

try {
    execFileSync('go', ['test', '-c', '-o', binary, './internal/server'], {cwd: root, stdio: 'inherit', timeout: 30000})
    fixture = spawn(binary, ['-test.run=^TestBrowserLiveFixture$', '-test.v'], {
        cwd: root,
        env: {...process.env, FAIRDROP_BROWSER_RECEIVER_LIVE: '1'},
        stdio: ['pipe', 'pipe', 'pipe'],
    })
    fixtureExited = new Promise((accept) => {
        fixture.once('error', (error) => accept({error}))
        fixture.once('exit', (code, signal) => accept({code, signal}))
    })
    fixture.stderr.on('data', (chunk) => { fixtureErrors += chunk.toString() })
    const ready = new Promise((accept) => {
        fixture.stdout.on('data', (chunk) => {
            fixtureOutput += chunk.toString()
            const found = fixtureOutput.match(/FAIRDROP_RECEIVER_URL=(http:\/\/[^\s]+)/)
            const long = fixtureOutput.match(/FAIRDROP_RECEIVER_LONG_URL=(http:\/\/[^\s]+)/)
            if (found && long) accept({url: found[1], longUrl: long[1]})
        })
    })
    const unexpectedExit = fixtureExited.then(({code, signal, error}) => { throw new Error(`receiver fixture exited before browser check completed: code=${code} signal=${signal} error=${error}\n${fixtureOutput}\n${fixtureErrors}`) })
    const urls = await within(Promise.race([ready, unexpectedExit]), 20000, 'receiver fixture readiness')
    await within(Promise.race([checkPage(urls), unexpectedExit]), 45000, 'receiver browser check')
} finally {
    await browser?.close()
    if (fixture) {
        if (fixture.pid && fixture.exitCode === null && fixture.signalCode === null) {
            fixture.stdin.end()
            try {
                await within(fixtureExited, 3000, 'receiver fixture cleanup')
            } catch {
                fixture.kill()
                await within(fixtureExited, 3000, 'receiver fixture forced cleanup')
            }
        }
    }
    rmSync(temp, {recursive: true, force: true})
}
