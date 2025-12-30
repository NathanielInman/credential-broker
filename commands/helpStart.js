import chalk from 'chalk'

export function helpStart() {
  console.log(chalk.green('Command: ') + chalk.white('start'))
  console.log(chalk.green('Brief: ') + chalk.white('Start the broker server'))
  console.log(chalk.green('Description: '))
  console.log(
    chalk.white(
      '  This command starts the broker server. Both the client and the server are the same code, what differentiates them is that the server needs to be started and listening while the client is a command line tool used to periodically query the server.'
    )
  )
  console.log(
    chalk.white(
      '  The broker server will first ask a series of questions to isolate restoration strategies before it will being listening. The operator will being using the service by first creating a user with the client instance and the command: '
    ) + chalk.cyan('broker init')
  )
}
