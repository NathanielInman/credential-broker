const chalk = require('chalk');
const {request} = require('../libraries/request.js');

module.exports = {
  async scopeGet(scopeName){
    try{
      const res = await request('scopeGet',{scopeName});

      if(res.success){
        console.log(chalk.green(JSON.stringify(res.success)));
      }else{
        console.log(chalk.red(JSON.stringify(res.error)));
      } //end if
    }catch(err){
      console.log(chalk.red('Problem connecting to server.'));
      console.log(chalk.red(err));
    }
  }
};
