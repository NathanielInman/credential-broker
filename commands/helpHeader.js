import chalk from 'chalk'
import fs from 'fs'
import { User } from '../models/User.js'

export function helpHeader() {
  let user

  if (!fs.existsSync('./user.json')) {
    user = new User()
  } else {
    user = new User(JSON.parse(fs.readFileSync('./user.json')))
  } // end if

  console.log(chalk.blue('         ./*.           '))
  console.log(chalk.blue('     /@@@@@@@@@@.       '))
  console.log(chalk.blue('   ,@@@@,    /@@@@      '))
  console.log(chalk.blue('  .@@@.        *@@@     '))
  console.log(chalk.blue('  @@@,          &@@%    '))
  console.log(chalk.blue('  @@@.          %@@%    '))
  console.log(chalk.blue('  @@@.          %@@%    '))
  console.log(chalk.blue('@@@@@@@@@@@@@@@@@@@@@@, ') + user.getVersionNumber())
  console.log(chalk.blue('@@@@@@@@@@%%@@@@@@@@@@, ') + user.getLoggedInRemoteIP())
  console.log(chalk.blue('@@@@@@@@#    ,@@@@@@@@, ') + user.getLoggedInUsername())
  console.log(chalk.blue('@@@@@@@@&    #@@@@@@@@, ') + user.getLoggedInEmail())
  console.log(chalk.blue('@@@@@@@@@.   @@@@@@@@@, ') + user.getLastAuthenticationDate())
  console.log(chalk.blue('@@@@@@@@@@#(@@@@@@@@@@, ') + user.getLastAction())
  console.log(chalk.blue('@@@@@@@@@@@@@@@@@@@@@@  ') + user.getLastScope())
  console.log('')
}
