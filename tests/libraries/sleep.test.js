import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// Import the CommonJS module
const { sleep } = require('../../libraries/sleep.js')

describe('libraries/sleep.js', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  describe('sleep()', () => {
    it('should return a promise', () => {
      const result = sleep(100)
      expect(result).toBeInstanceOf(Promise)
    })

    it('should resolve after the specified delay', async () => {
      const callback = vi.fn()

      sleep(1000).then(callback)

      expect(callback).not.toHaveBeenCalled()

      await vi.advanceTimersByTimeAsync(999)
      expect(callback).not.toHaveBeenCalled()

      await vi.advanceTimersByTimeAsync(1)
      expect(callback).toHaveBeenCalled()
    })

    it('should resolve with undefined', async () => {
      const promise = sleep(100)
      vi.advanceTimersByTime(100)
      const result = await promise
      expect(result).toBeUndefined()
    })

    it('should handle zero delay', async () => {
      const callback = vi.fn()
      sleep(0).then(callback)

      await vi.advanceTimersByTimeAsync(0)
      expect(callback).toHaveBeenCalled()
    })

    it('should handle multiple concurrent sleeps', async () => {
      const order = []

      sleep(100).then(() => order.push('first'))
      sleep(200).then(() => order.push('second'))
      sleep(50).then(() => order.push('third'))

      await vi.advanceTimersByTimeAsync(200)

      expect(order).toEqual(['third', 'first', 'second'])
    })
  })
})
