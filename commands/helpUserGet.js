const chalk = require('chalk');

module.exports = {
  helpUserGet(){
    console.log(chalk.green('Command: ')+chalk.white('user get $username'));
    console.log(chalk.green('Brief: ')+chalk.white('Get a specific user and all information attached'));
    console.log(chalk.green('Description: '));
    console.log(chalk.white('  If an operator has user edit access they can use this command to retrieve all information attached to a user.'));
    console.log();
    console.log(chalk.green('Command: ')+chalk.white('user get'));
    console.log(chalk.green('Brief: ')+chalk.white('Get a list of all usernames'));
    console.log(chalk.green('Description: '));
    console.log(chalk.white('  If an operator has user view access they can use this command to retrieve a list of all existing users on the broker service.'));

  }
}
