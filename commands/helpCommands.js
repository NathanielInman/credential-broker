const chalk = require('chalk');

module.exports = {
  helpCommands(){
    console.log(chalk.green('Commands: (')+chalk.white('help $command')+chalk.green(' for more information)'));
    console.log(chalk.white('  start                      ')+chalk.green('start broker server'));
    console.log(chalk.white('  init                       ')+chalk.green('initialize client and log into user'));
    console.log(chalk.white('  wipe                       ')+chalk.green('clear all the users on the service'));
    console.log(chalk.white('  user add $username         ')+chalk.green('create a new user'));
    console.log(chalk.white('  user del $username         ')+chalk.green('delete an existing user'));
    console.log(chalk.white('  user mod $username         ')+chalk.green('modify an existing user'));
    console.log(chalk.white('  user get                   ')+chalk.green('get a list of all users'));
    console.log(chalk.white('  user get $username         ')+chalk.green('get information on existing user'));
    console.log(chalk.white('  scope add $scopename       ')+chalk.green('create a new scope'));
    console.log(chalk.white('  scope del $scopename       ')+chalk.green('delete an existing scope'));
    console.log(chalk.white('  scope mod $scopename       ')+chalk.green('modify an existing scope'));
    console.log(chalk.white('  scope get                  ')+chalk.green('get a list of all scope names'));
    console.log(chalk.white('  get                        ')+chalk.green('get a list of all scope names'));
    console.log(chalk.white('  add $scopename $secretname ')+chalk.green('create a new secret on existing scope'));
    console.log(chalk.white('  del $scopename $secretname ')+chalk.green('delete an existing secret on existing scope'));
    console.log(chalk.white('  mod $scopename $secretname ')+chalk.green('modify an existing secret on existing scope'));
    console.log(chalk.white('  get $scopename             ')+chalk.green('get all secrets from an existing scope'));
    console.log(chalk.green('Other Help:'));
    console.log(chalk.white('  help abandonment'));
  }
}
