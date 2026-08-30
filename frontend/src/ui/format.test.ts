import {describe, expect, it} from 'vitest'
import {formatBytes, formatRate} from './format'

describe('byte presentation', () => {
    it('counts exactly below one kilobyte and agrees with itself on the singular', () => {
        expect(formatBytes(0)).toBe('0 bytes')
        expect(formatBytes(1)).toBe('1 byte')
        expect(formatBytes(2)).toBe('2 bytes')
        expect(formatBytes(999)).toBe('999 bytes')
    })

    it('scales in decimal steps with one decimal place', () => {
        expect(formatBytes(1000)).toBe('1.0 KB')
        expect(formatBytes(1500)).toBe('1.5 KB')
        expect(formatBytes(8_400_000)).toBe('8.4 MB')
        expect(formatBytes(36_800_000)).toBe('36.8 MB')
        expect(formatBytes(2_000_000_000)).toBe('2.0 GB')
        expect(formatBytes(1_000_000_000_000)).toBe('1.0 TB')
    })

    it('carries a value that rounds up to a full unit into the next unit', () => {
        // 999 999 bytes is 999.999 KB, which would otherwise print "1000.0 KB".
        expect(formatBytes(999_999)).toBe('1.0 MB')
    })

    it('reports zero rather than a negative, fractional or non-finite count', () => {
        expect(formatBytes(-1)).toBe('0 bytes')
        expect(formatBytes(Number.NaN)).toBe('0 bytes')
        expect(formatBytes(Number.POSITIVE_INFINITY)).toBe('0 bytes')
        expect(formatBytes(0.4)).toBe('0 bytes')
        expect(formatBytes(1.9)).toBe('1 byte')
    })
})

describe('throughput presentation', () => {
    it('appends the rate suffix to the same byte scale', () => {
        expect(formatRate(0)).toBe('0 bytes/s')
        expect(formatRate(4_700_000)).toBe('4.7 MB/s')
        expect(formatRate(12_400_000)).toBe('12.4 MB/s')
    })
})
