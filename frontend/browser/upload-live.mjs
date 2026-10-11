import {spawn, execFileSync} from 'node:child_process'
import {mkdtempSync, readdirSync, readFileSync, rmSync} from 'node:fs'
import {tmpdir} from 'node:os'
import {join, resolve} from 'node:path'
import {fileURLToPath} from 'node:url'
import {chromium} from 'playwright'

// Drives the real production receive route in a real browser: the script-free
// upload page, a real multipart form submission with several files (two of them
// with the same name), and the bytes that reach the disk. Chromium only -- the
// WKWebView and mobile-browser paths are not proven here.

const root = resolve(fileURLToPath(new URL('../..', import.meta.url)))
const temp = mkdtempSync(join(tmpdir(), 'fairdrop-upload-'))
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

async function checkPage({url, dir}) {
    browser = await chromium.launch({headless: true})
    const context = await browser.newContext({viewport: {width: 320, height: 700}})
    const page = await context.newPage()
    const response = await page.goto(url)
    if (response?.status() !== 200) throw new Error(`GET status = ${response?.status()}`)
    const headers = response.headers()
    if (headers['content-security-policy'] !== "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'") throw new Error('live CSP differs from receiver policy')
    if (headers['referrer-policy'] !== 'no-referrer' || headers['cache-control'] !== 'no-store') throw new Error('live privacy headers missing')
    if (await page.locator('h1').textContent() !== 'Send files to this computer') throw new Error('upload heading missing')
    if (await page.locator('script').count() !== 0) throw new Error('the upload page contains a script')
    if (await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)) throw new Error('page scrolls horizontally at 320px')
    const input = page.locator('input[type=file][name=files]')
    if (!(await input.evaluate((element) => element.multiple && element.required))) throw new Error('the file input is not a required multiple input')
    const button = page.getByRole('button', {name: 'Upload'})
    if (await button.evaluate((element) => getComputedStyle(element).minHeight) !== '48px') throw new Error('Upload target too small')

    // Submitting with nothing chosen is stopped by the browser: no request is sent.
    let posts = 0
    page.on('request', (request) => { if (request.method() === 'POST') posts++ })
    await button.click()
    await page.waitForTimeout(300)
    if (posts !== 0) throw new Error('an empty form was submitted')

    await input.setInputFiles([
        {name: 'IMG_0001.jpg', mimeType: 'image/jpeg', buffer: Buffer.from('first photo bytes')},
        {name: 'IMG_0001.jpg', mimeType: 'image/jpeg', buffer: Buffer.from('second photo bytes')},
        {name: 'notes.txt', mimeType: 'text/plain', buffer: Buffer.from('plain notes')},
    ])
    const [result] = await Promise.all([
        page.waitForResponse((r) => r.request().method() === 'POST'),
        button.click(),
    ])
    if (result.status() !== 200) throw new Error(`POST status = ${result.status()}`)
    await page.waitForLoadState('load')
    if ((await page.locator('h1').textContent()) !== 'Upload complete') throw new Error('result heading missing')
    if (!(await page.locator('.detail').textContent()).includes('3 files saved to this computer.')) throw new Error('result page does not state 3 files saved')
    const text = await page.locator('body').innerText()
    if (text.includes(dir) || text.includes('IMG_0001')) throw new Error('the result page disclosed a path or file name')

    const subfolders = readdirSync(dir).filter((name) => name.startsWith('FairDrop '))
    if (subfolders.length !== 1) throw new Error(`destination holds ${readdirSync(dir)}, want exactly one FairDrop subfolder`)
    const saved = join(dir, subfolders[0])
    const names = readdirSync(saved).sort()
    if (names.join('|') !== 'IMG_0001 (1).jpg|IMG_0001.jpg|notes.txt') throw new Error(`saved ${names}`)
    if (readFileSync(join(saved, 'IMG_0001.jpg'), 'utf8') !== 'first photo bytes') throw new Error('first photo bytes differ')
    if (readFileSync(join(saved, 'IMG_0001 (1).jpg'), 'utf8') !== 'second photo bytes') throw new Error('second photo bytes differ')
    if (readFileSync(join(saved, 'notes.txt'), 'utf8') !== 'plain notes') throw new Error('notes bytes differ')
    console.log('upload live browser: real page, headers, 320px layout, empty form blocked, multipart POST of 3 files and exact saved bytes passed')
}

try {
    execFileSync('go', ['test', '-c', '-o', binary, './internal/server'], {cwd: root, stdio: 'inherit', timeout: 60000})
    fixture = spawn(binary, ['-test.run=^TestBrowserUploadFixture$', '-test.v'], {
        cwd: join(root, 'internal/server'),
        env: {...process.env, FAIRDROP_BROWSER_UPLOAD_LIVE: '1'},
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
            const url = fixtureOutput.match(/FAIRDROP_UPLOAD_URL=(http:\/\/[^\s]+)/)
            const dir = fixtureOutput.match(/FAIRDROP_UPLOAD_DIR=([^\r\n]+)/)
            if (url && dir) accept({url: url[1], dir: dir[1]})
        })
    })
    const unexpectedExit = fixtureExited.then(({code, signal, error}) => { throw new Error(`upload fixture exited before browser check completed: code=${code} signal=${signal} error=${error}\n${fixtureOutput}\n${fixtureErrors}`) })
    const target = await within(Promise.race([ready, unexpectedExit]), 30000, 'upload fixture readiness')
    await within(Promise.race([checkPage(target), unexpectedExit]), 45000, 'upload browser check')
} finally {
    await browser?.close()
    if (fixture) {
        if (fixture.pid && fixture.exitCode === null && fixture.signalCode === null) {
            fixture.stdin.end()
            try {
                await within(fixtureExited, 3000, 'upload fixture cleanup')
            } catch {
                fixture.kill()
                await within(fixtureExited, 3000, 'upload fixture forced cleanup')
            }
        }
    }
    rmSync(temp, {recursive: true, force: true})
}
