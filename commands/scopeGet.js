const chalk = require('chalk');
const {request} = require('../libraries/request.js');

module.exports = {
  async scopeGet(scopeName){
    try{
      const res = request('scopeGet',{scopeName});

      if(res.success){
        console.log(chalk.green(res.success));
      }else{
        console.log(chalk.red(res.error));
      } //end if
    }catch(err){
      console.log(chalk.red('Problem connecting to server.'));
      console.log(chalk.red(err));
    }
  }
};
