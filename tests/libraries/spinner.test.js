import { describe, it, expect } from 'vitest'

const { spinner } = require('../../libraries/spinner.js')

describe('libraries/spinner.js', () => {
  describe('spinner export', () => {
    it('should export a spinner object', () => {
      expect(spinner).toBeDefined()
      expect(spinner).toBeTypeOf('object')
    })

    it('should have setSpinnerString method', () => {
      expect(spinner.setSpinnerString).toBeDefined()
      expect(typeof spinner.setSpinnerString).toBe('function')
    })

    it('should have setSpinnerTitle method', () => {
      expect(spinner.setSpinnerTitle).toBeDefined()
      expect(typeof spinner.setSpinnerTitle).toBe('function')
    })

    it('should have start method', () => {
      expect(spinner.start).toBeDefined()
      expect(typeof spinner.start).toBe('function')
    })

    it('should have stop method', () => {
      expect(spinner.stop).toBeDefined()
      expect(typeof spinner.stop).toBe('function')
    })

    it('should allow setting spinner title without error', () => {
      expect(() => spinner.setSpinnerTitle('Test')).not.toThrow()
    })

    it('should allow setting spinner string without error', () => {
      expect(() => spinner.setSpinnerString('|/-\\')).not.toThrow()
    })
  })
})
