const fs = require('fs');
const {confirm} = require('../libraries/prompt.js');
const chalk = require('chalk');
const {User} = require('../models/User.js');

module.exports = {
  async init(){
    if(fs.existsSync('./user.json')){
      const user = new User(JSON.parse(fs.readFileSync('./user.json')));

      if(await confirm(`User (${chalk.magenta(user.name)}) exists already, remove?`)){
        fs.unlinkSync('./user.json');
        await user.initialize();
      }else{
        console.log('Canceling initialization');
        process.exit(0);
      } //end if
    }else{
      const user = new User();

      await user.initialize();
    } //end if
  }
};
