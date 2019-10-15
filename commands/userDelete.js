const chalk = require('chalk');
const {request} = require('../libraries/request.js');

module.exports = {
  async userDelete(name){
    try{
      const res = await request('userDelete',{name});

      if(res.success){
        console.log(chalk.green(`User "${name}" deleted successfully!`));
      }else{
        console.log(chalk.red(res.error));
      } //end if
    }catch(err){
      console.log(chalk.red('Problem connecting to server.'));
      console.log(chalk.red(err));
    }
  }
};
