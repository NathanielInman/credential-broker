import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const { log } = require('../../libraries/log.js')

describe('libraries/log.js', () => {
  let consoleSpy

  beforeEach(() => {
    consoleSpy = vi.spyOn(console, 'log').mockImplementation(() => {})
  })

  afterEach(() => {
    consoleSpy.mockRestore()
  })

  describe('log()', () => {
    it('should log with default parameters', () => {
      log()

      expect(consoleSpy).toHaveBeenCalledTimes(1)
      const output = consoleSpy.mock.calls[0][0]
      expect(output).toContain('unknown ip')
      expect(output).toContain('unknown name')
      expect(output).toContain('unknown event')
    })

    it('should log with custom ip', () => {
      log('192.168.1.1')

      const output = consoleSpy.mock.calls[0][0]
      expect(output).toContain('192.168.1.1')
    })

    it('should log with custom name', () => {
      log('127.0.0.1', 'testuser')

      const output = consoleSpy.mock.calls[0][0]
      expect(output).toContain('testuser')
    })

    it('should log with custom description', () => {
      log('127.0.0.1', 'admin', 'User login')

      const output = consoleSpy.mock.calls[0][0]
      expect(output).toContain('User login')
    })

    it('should include FAILURE marker when failure is true', () => {
      log('127.0.0.1', 'admin', 'Login failed', true)

      const output = consoleSpy.mock.calls[0][0]
      expect(output).toContain('FAILURE')
    })

    it('should not include FAILURE marker when failure is false', () => {
      log('127.0.0.1', 'admin', 'Login success', false)

      const output = consoleSpy.mock.calls[0][0]
      expect(output).not.toContain('FAILURE')
    })

    it('should call console.log exactly once per call', () => {
      log('10.0.0.1', 'user1', 'Test event', false)
      log('10.0.0.2', 'user2', 'Another event', true)

      expect(consoleSpy).toHaveBeenCalledTimes(2)
    })

    it('should include all provided parameters in output', () => {
      log('10.0.0.1', 'specialuser', 'Special event', true)

      const output = consoleSpy.mock.calls[0][0]
      expect(output).toContain('10.0.0.1')
      expect(output).toContain('specialuser')
      expect(output).toContain('Special event')
      expect(output).toContain('FAILURE')
    })
  })
})
