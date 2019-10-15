const chalk = require('chalk');
const {request} = require('../libraries/request.js');

module.exports = {
  async scopeDelete(scopeName){
    try{
      const res = request('scopeDelete',{scopeName});

      if(res.success){
        console.log(chalk.green(`Scope "${scopeName}" deleted successfully!`));
      }else{
        console.log(chalk.red(res.error));
      } //end if
    }catch(err){
      console.log(chalk.red('Problem connecting to server.'));
      console.log(chalk.red(err));
    }
  }
};
