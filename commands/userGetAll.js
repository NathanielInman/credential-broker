const chalk = require('chalk');
const {request} = require('../libraries/request.js');

module.exports = {
  async userGetAll(){
    try{
      const res = await request('userGetAll');

      if(res.success){
        console.log(chalk.magenta('[USERS]'));
        res.success.forEach(userName=> console.log(chalk.green(userName)));
      }else{
        console.log(chalk.red(res.error));
      } //end if
    }catch(err){
      console.log(chalk.red('Problem connecting to server.'));
      console.log(chalk.red(err));
    }
  }
};
