import chalk from 'chalk'

export function log(
  ip = 'unknown ip',
  name = 'unknown name',
  description = 'unknown event',
  failure = false
) {
  console.log(
    chalk.cyan(`[${ip}]`) +
      chalk.magenta(`<${name}>`) +
      chalk.grey(':') +
      (failure ? chalk.red(' [FAILURE] ') : '') +
      chalk.green(` ${description}`)
  )
}
