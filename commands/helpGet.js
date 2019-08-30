const chalk = require('chalk');

module.exports = {
  helpGet(){
    console.log(chalk.green('Command: ')+chalk.white('get')+chalk.cyan(' or ')+chalk.white('scope get'));
    console.log(chalk.green('Brief: ')+chalk.white('Get a list of all scope names'));
    console.log(chalk.green('Description: '));
    console.log(chalk.white('  Get a list of all scope names if operator has access to viewing all scope names.'));
    console.log('');
    console.log(chalk.green('Command: ')+chalk.white('get $scopename'));
    console.log(chalk.green('Brief: ')+chalk.white('Get all secrets attached to a scope'));
    console.log(chalk.green('Description: '));
    console.log(chalk.white('  The operator or application may retrieve all secrets attached to a scope if they have view access to that scope. The values are streamed from the broker and it\'s recommended that they are injected directly into an application runtime so they are lost upon application termination and aren\'t visible in plaintext on disk.'));

  }
}
