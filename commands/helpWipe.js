const chalk = require('chalk');

module.exports = {
  helpWipe(){
    console.log(chalk.green('Command: ')+chalk.white('wipe'));
    console.log(chalk.green('Brief: ')+chalk.white('Trigger a wipe event to clear all users'));
    console.log(chalk.green('Description: '));
    console.log(chalk.white('  Anyone with access to the server containing the broker service may trigger a wipe event. By default it gives 24hour warning to all users that a wipe will occur. This warning may be disabled. See: '+chalk.cyan('help abandonment')));
  }
}
