const chalk = require('chalk');
const {request} = require('../libraries/request.js');
const {prompt,confirm,password} = require('../libraries/prompt.js');

module.exports = {
  async secretModify(scopeName,secretName){
    let bool;

    do{
      bool = await confirm(chalk.green(`Is this the correct secret name: "${secretName}"?`));
      if(!bool){
        console.log(chalk.green('No problem, let\'s try again.'));
        secretName = await prompt(chalk.green('Enter secret name: '));
      } //end if
    }while(!bool)
    const secretValue = await password(chalk.green(`Please enter the updated secret value (concealed): `));
    try{
      const res = await request('secretModify',{scopeName,secretName,secretValue});

      if(res.success){
        console.log(chalk.green(`Secret "${secretName}" from "${scopeName}" modified successfully!`));
      }else{
        console.log(chalk.red(res.error));
      } //end if
    }catch(err){
      console.log(chalk.red('Problem connecting to server.'));
      console.log(chalk.red(err));
    }
  }
};
