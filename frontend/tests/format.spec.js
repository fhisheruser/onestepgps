import { describe, expect, it } from 'vitest'
import {
  convertSpeed,
  driveStatusMeta,
  formatCoords,
  formatDuration,
  formatOdometer,
  formatSpeed,
  headingToCompass,
  timeAgo,
  truncate,
} from '@/utils/format'

describe('convertSpeed', () => {
  it('converts between the units the provider actually reports', () => {
    expect(convertSpeed(100, 'km/h', 'mph')).toBeCloseTo(62.137, 2)
    expect(convertSpeed(60, 'mph', 'kph')).toBeCloseTo(96.56, 1)
    expect(convertSpeed(10, 'm/s', 'kph')).toBeCloseTo(36, 5)
    expect(convertSpeed(50, 'kph', 'kph')).toBe(50)
  })

  it('passes unknown units through instead of inventing a number', () => {
    expect(convertSpeed(42, 'furlongs/fortnight', 'mph')).toBe(42)
    expect(convertSpeed(42, 'kph', 'parsecs')).toBe(42)
  })

  it('treats missing speeds as zero', () => {
    expect(convertSpeed(undefined, 'kph', 'mph')).toBe(0)
    expect(convertSpeed(Number.NaN, 'kph', 'mph')).toBe(0)
  })
})

describe('formatSpeed', () => {
  it('labels the unit the user chose, not the one the provider sent', () => {
    expect(formatSpeed(100, 'km/h', 'mph')).toBe('62.1 mph')
    expect(formatSpeed(0, 'km/h', 'kph')).toBe('0 km/h')
  })

  it('drops the decimal once the number is large enough not to need it', () => {
    expect(formatSpeed(200, 'kph', 'kph')).toBe('200 km/h')
  })
})

describe('timeAgo', () => {
  const now = Date.parse('2026-08-11T12:00:00Z')

  it('describes recent timestamps in the units a dispatcher thinks in', () => {
    expect(timeAgo('2026-08-11T11:59:57Z', now)).toBe('just now')
    expect(timeAgo('2026-08-11T11:59:30Z', now)).toBe('30s ago')
    expect(timeAgo('2026-08-11T11:45:00Z', now)).toBe('15m ago')
    expect(timeAgo('2026-08-11T09:00:00Z', now)).toBe('3h ago')
    expect(timeAgo('2026-08-08T12:00:00Z', now)).toBe('3d ago')
  })

  it('does not crash on missing or malformed input', () => {
    expect(timeAgo(null, now)).toBe('never')
    expect(timeAgo('not-a-date', now)).toBe('unknown')
  })
})

describe('misc formatters', () => {
  it('formats durations compactly', () => {
    expect(formatDuration(45)).toBe('45s')
    expect(formatDuration(600)).toBe('10m')
    expect(formatDuration(9000)).toBe('2h 30m')
    expect(formatDuration(0)).toBe('—')
  })

  it('formats coordinates to metre precision and rejects junk', () => {
    expect(formatCoords(32.715736, -117.161087)).toBe('32.71574, -117.16109')
    expect(formatCoords(undefined, -117)).toBe('—')
  })

  it('maps headings onto compass points', () => {
    expect(headingToCompass(0)).toBe('N')
    expect(headingToCompass(90)).toBe('E')
    expect(headingToCompass(359)).toBe('N')
    expect(headingToCompass(-90)).toBe('W')
  })

  it('formats odometers and hides empty ones', () => {

    expect(formatOdometer(123456.7, 'km')).toBe(`${(123457).toLocaleString()} km`)
    expect(formatOdometer(0, 'km')).toBe('—')
  })

  it('truncates on a word boundary', () => {
    expect(truncate('short', 20)).toBe('short')
    expect(truncate('refrigerated trailer needs servicing', 20)).toBe('refrigerated trailer…')
  })

  it('falls back to a known status for anything unexpected', () => {
    expect(driveStatusMeta('driving').label).toBe('Driving')
    expect(driveStatusMeta('nonsense').label).toBe('Unknown')
    expect(driveStatusMeta(undefined).label).toBe('Unknown')
  })
})
