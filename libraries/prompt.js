import keypress from 'keypress'

export function prompt(msg) {
  return new Promise((resolve) => {
    process.stdout.write(msg)
    process.stdin.setEncoding('utf8')
    process.stdin
      .once('data', (val) => {
        resolve(val.trim())
      })
      .resume()
  })
    .then((result) => {
      process.stdin.pause()
      return result
    })
    .catch((e) => {
      process.stdin.pause()
      return Promise.reject(e)
    })
}

export function multiline(msg) {
  return new Promise((resolve) => {
    const buf = []
    console.log(msg)
    process.stdin.setEncoding('utf8')
    process.stdin
      .on('data', (val) => {
        if (val === '\n' || val === '\r\n') {
          process.stdin.removeAllListeners('data')
          resolve(buf.join('\n'))
        } else {
          buf.push(val.trimEnd())
        }
      })
      .resume()
  })
    .then((result) => {
      process.stdin.pause()
      return result
    })
    .catch((e) => {
      process.stdin.pause()
      return Promise.reject(e)
    })
}

export function confirm(msg) {
  return prompt(`${msg} [Y/n]`).then((val) => {
    return !val.length || /^y|ye|yes|ok|true$/i.test(val.toLowerCase())
  })
}

export function password(msg, mask = '*') {
  const isTTY = process.stdin.isTTY

  if (!isTTY) return prompt(msg)

  return new Promise((resolve) => {
    let buf = ''

    keypress(process.stdin)

    process.stdin.setRawMode(true)
    process.stdout.write(msg)

    process.stdin
      .on('keypress', (c, key) => {
        if (key && key.name === 'return') {
          console.log()
          process.stdin.pause()
          process.stdin.removeAllListeners('keypress')
          process.stdin.setRawMode(false)
          if (!buf.trim().length) {
            return password(msg, mask).then(resolve)
          }
          resolve(buf)
          return
        }

        if (key && key.ctrl && key.name === 'c') {
          console.log('%s', buf)
          process.exit()
        }

        process.stdout.write(mask)
        buf += c
      })
      .resume()
  })
    .then((result) => {
      process.stdin.pause()
      return result
    })
    .catch((e) => {
      process.stdin.pause()
      return Promise.reject(e)
    })
}
