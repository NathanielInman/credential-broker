const chalk = require('chalk');
const fs = require('fs');
const {BrokerState} = require('../models/BrokerState');

if(!fs.existsSync('./state.json')){
  fs.writeFileSync('./state.json',JSON.stringify(new BrokerState));
} //end if
const state = new BrokerState(JSON.parse(fs.readFileSync('./state.json')));

module.exports = {
  helpHeader(){
    console.log(chalk.blue( '         ./*.           '));
    console.log(chalk.blue( '     /@@@@@@@@@@.       '));
    console.log(chalk.blue( '   ,@@@@,    /@@@@      '));
    console.log(chalk.blue( '  .@@@.        *@@@     '));
    console.log(chalk.blue( '  @@@,          &@@%    '));
    console.log(chalk.blue( '  @@@.          %@@%    '));
    console.log(chalk.blue( '  @@@.          %@@%    '));
    console.log(chalk.blue( '@@@@@@@@@@@@@@@@@@@@@@, ')+state.getVersionNumber());
    console.log(chalk.blue( '@@@@@@@@@@%%@@@@@@@@@@, ')+state.getLoggedInRemoteIP());
    console.log(chalk.blue( '@@@@@@@@#    ,@@@@@@@@, ')+state.getLoggedInUsername());
    console.log(chalk.blue( '@@@@@@@@&    #@@@@@@@@, ')+state.getLoggedInEmail());
    console.log(chalk.blue( '@@@@@@@@@.   @@@@@@@@@, ')+state.getLastAuthenticationDate());
    console.log(chalk.blue( '@@@@@@@@@@#(@@@@@@@@@@, ')+state.getLastAction());
    console.log(chalk.blue( '@@@@@@@@@@@@@@@@@@@@@@  ')+state.getLastScope());
    console.log('');
  }
};
