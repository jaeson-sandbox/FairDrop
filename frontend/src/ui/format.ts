import {copy} from './copy'

/**
 * Byte and rate presentation for the item summary and the transfer metrics.
 *
 * Decimal units, because the receiver browser's own download UI reports decimal
 * and the two numbers sit side by side on the user's screen. Unit words come
 * from the copy registry like every other literal.
 *
 * These format a value the backend measured; they never derive one. Nothing
 * here divides a wire count by a total or turns a logical size into progress.
 */

const scaledUnits = [
    copy.unit.kilobytes,
    copy.unit.megabytes,
    copy.unit.gigabytes,
    copy.unit.terabytes,
] as const

const step = 1000

/** Renders a byte count, exact below 1 KB and to one decimal above it. */
export function formatBytes(value: number): string {
    if (!Number.isFinite(value) || value <= 0) return `0 ${copy.unit.bytes}`

    const whole = Math.floor(value)
    if (whole < step) {
        if (whole === 0) return `0 ${copy.unit.bytes}`
        return `${whole} ${whole === 1 ? copy.unit.byte : copy.unit.bytes}`
    }

    let scaled = value / step
    let index = 0
    // Rounding is applied before the comparison so 999 999 bytes reads
    // "1.0 MB" rather than the "1000.0 KB" a raw magnitude test would leave.
    while (index < scaledUnits.length - 1 && Number(scaled.toFixed(1)) >= step) {
        scaled /= step
        index += 1
    }
    return `${scaled.toFixed(1)} ${scaledUnits[index]}`
}

/** Renders a throughput. Visual only: it is never spoken and never a total. */
export function formatRate(bytesPerSecond: number): string {
    return `${formatBytes(bytesPerSecond)}${copy.unit.perSecond}`
}
