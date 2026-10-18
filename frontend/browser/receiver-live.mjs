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

async function checkPage({url, longUrl, textUrl, receiveTextUrl}) {
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
    const textContext = await browser.newContext({viewport: {width: 320, height: 700}, javaScriptEnabled: false})
    const textPage = await textContext.newPage()
    const textPreview = await textPage.goto(textUrl)
    if (textPreview?.status() !== 200 || await textPage.locator('textarea').count() !== 0) throw new Error('text GET disclosed payload or failed')
    if ((await textPage.locator('body').textContent()).includes('A <b>literal</b> URL')) throw new Error('text GET disclosed content')
    if (await textPage.evaluate(() => document.documentElement.scrollWidth > innerWidth)) throw new Error('text preview scrolls horizontally at 320px')
    await textPage.getByRole('button', {name: 'Read text'}).click()
    if (await textPage.locator('h1').textContent() !== 'Text from FairDrop') throw new Error('text reveal failed without scripting')
    const textField = textPage.getByRole('textbox', {name: 'Shared text'})
    if (await textField.inputValue() !== '\nA <b>literal</b> URL: https://example.test/?a=1&b=2') throw new Error('text reveal DOM value changed')
    if (await textField.getAttribute('readonly') === null || await textPage.locator('script,a[href]').count() !== 0) throw new Error('text reveal is active or editable')
    if (await textPage.evaluate(() => document.documentElement.scrollWidth > innerWidth)) throw new Error('text reveal scrolls horizontally at 320px')
    await textField.focus()
    await textPage.keyboard.press('ControlOrMeta+A')
    const selection = await textField.evaluate((element) => ({start: element.selectionStart, end: element.selectionEnd, length: element.value.length}))
    if (selection.start !== 0 || selection.end !== selection.length) throw new Error(`native text selection failed without scripting: ${JSON.stringify(selection)}`)
    await textPage.emulateMedia({forcedColors: 'active'})
    if (await textField.evaluate((element) => getComputedStyle(element).outlineStyle) === 'none') throw new Error('text reveal focus has no forced-colors outline')
    await textContext.close()
    const inboundContext = await browser.newContext({viewport: {width: 320, height: 700}, javaScriptEnabled: false})
    const inbound = await inboundContext.newPage()
    const inboundResponse = await inbound.goto(receiveTextUrl)
    if (inboundResponse?.status() !== 200) throw new Error(`receive-text GET status=${inboundResponse?.status()}`)
    const inboundHeaders = inboundResponse.headers()
    if (inboundHeaders['cache-control'] !== 'no-store' || inboundHeaders['referrer-policy'] !== 'no-referrer' ||
        inboundHeaders['content-security-policy'] !== "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'") {
        throw new Error('receive-text privacy headers missing')
    }
    if (await inbound.evaluate(() => document.documentElement.scrollWidth > innerWidth)) throw new Error('receive-text form scrolls at 320px')
    if (await inbound.locator('script').count() !== 0 || await inbound.locator('form').count() !== 1 ||
        await inbound.locator('form textarea[name="text"]').count() !== 1) throw new Error('production phone form is missing or duplicated')
    const form = inbound.locator('form')
    if (await form.getAttribute('method') !== 'post' || await form.getAttribute('action') !== '' ||
        await form.getAttribute('enctype') !== 'application/x-www-form-urlencoded') throw new Error('phone form submission contract changed')
    const input = inbound.getByRole('textbox', {name: 'Text to send'})
    const send = inbound.getByRole('button', {name: 'Send text'})
    if (await send.evaluate((element) => element.getBoundingClientRect().height) < 44) throw new Error('phone submit target is below 44px')
    await inbound.keyboard.press('Tab')
    if (!(await input.evaluate((element) => element === document.activeElement))) throw new Error('phone text field is not keyboard reachable')
    if (await input.evaluate((element) => getComputedStyle(element).outlineStyle) === 'none') throw new Error('phone input has no focus outline')
    await inbound.emulateMedia({colorScheme: 'dark'})
    if (await inbound.evaluate(() => getComputedStyle(document.documentElement).backgroundColor) !== 'rgb(22, 22, 24)') throw new Error('phone dark canvas missing')
    await inbound.emulateMedia({forcedColors: 'active'})
    if (await input.evaluate((element) => getComputedStyle(element).outlineStyle) === 'none') throw new Error('phone input has no forced-colors outline')
    await inbound.emulateMedia({forcedColors: 'none'})
    const value = 'First line\n雪 & <tag> https://example.test/?a=1&b=2'
    await input.fill(value)
    await input.press('ControlOrMeta+A')
    if (await input.evaluate((element) => element.selectionStart !== 0 || element.selectionEnd !== element.value.length)) {
        throw new Error('phone form does not support native selection')
    }
    const submitted = inbound.waitForRequest(request => request.url() === receiveTextUrl && request.method() === 'POST')
    await send.click()
    const post = await submitted
    if (post.headers()['content-type'] !== 'application/x-www-form-urlencoded') throw new Error('phone form content type changed')
    const body = post.postData() ?? ''
    if (!body.includes('%0D%0A') || new URLSearchParams(body).getAll('text').length !== 1 ||
        new URLSearchParams(body).get('text') !== value.replaceAll('\n', '\r\n')) throw new Error(`phone browser form encoding changed: ${body}`)
    if (await inbound.locator('h1').textContent() !== 'Text received by this computer') throw new Error('phone submission acknowledgement missing')
    await inboundContext.close()
    console.log('receiver live browser: file download, script-free text reveal and production receive-text form at 320px passed')
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
            const text = fixtureOutput.match(/FAIRDROP_RECEIVER_TEXT_URL=(http:\/\/[^\s]+)/)
            const receiveText = fixtureOutput.match(/FAIRDROP_RECEIVER_RECEIVE_TEXT_URL=(http:\/\/[^\s]+)/)
            if (found && long && text && receiveText) accept({url: found[1], longUrl: long[1], textUrl: text[1], receiveTextUrl: receiveText[1]})
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
            let result
            try {
                result = await within(fixtureExited, 3000, 'receiver fixture cleanup')
            } catch {
                fixture.kill()
                result = await within(fixtureExited, 3000, 'receiver fixture forced cleanup')
            }
            if (result.code !== 0) throw new Error(`receiver fixture failed: ${fixtureOutput}\n${fixtureErrors}`)
        }
    }
    rmSync(temp, {recursive: true, force: true})
}
