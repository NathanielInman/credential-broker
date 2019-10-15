const chalk = require('chalk');
const {request} = require('../libraries/request.js');
const {prompt,confirm,password} = require('../libraries/prompt.js');

module.exports = {
  async secretAdd(scopeName,secretName){
    let bool;

    do{
      bool = await confirm(chalk.green(`Is this the correct scope name: "${scopeName}"?`));
      if(!bool){
        console.log(chalk.green('No problem, let\'s try again.'));
        scopeName = prompt(chalk.green('Enter scope name: '));
      } //end if
    }while(!bool)
    do{
      bool = await confirm(chalk.green(`Is this the correct secret name: "${secretName}"?`));
      if(!bool){
        console.log(chalk.green('No problem, let\'s try again.'));
        secretName = await prompt(chalk.green('Enter new secret name: '));
      } //end if
    }while(!bool)
    const secretValue = await password(chalk.green(`Please enter the secret value (concealed): `));

    try{
      const res = await request('secretAdd',{scopeName,secretName,secretValue});

      if(res.success){
        console.log(chalk.green(`Secret "${secretName}" added to "${scopeName}" successfully!`));
      }else{
        console.log(chalk.red(res.error));
      } //end if
    }catch(err){
      console.log(chalk.red('Problem connecting to server.'));
      console.log(chalk.red(err));
    }
  }
};
