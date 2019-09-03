const fetch = require('node-fetch');
const fs = require('fs');
const chalk = require('chalk');
const {User} = require('../models/User.js');
const {prompt,confirm,password} = require('../libraries/prompt.js');

module.exports = {
  async secretModify(scopeName,secretName){

    //short-circuit failure
    if(!fs.existsSync('./user.json')){
      return console.log(chalk.red('No user exists locally, initialize first with: ')+chalk.cyan('broker init'));
    } //end if
    const user = new User(JSON.parse(fs.readFileSync('./user.json')));

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
      await fetch(`${user.remoteIP}/secretModify`,{
        method: 'POST',
        body: JSON.stringify({
          key: fs.readFileSync(user.pgpPrivateKeyLocation).toString(),
          scopeName,
          secretName,
          secretValue
        }),
        headers: {
          'Content-Type': 'application/json',
          name: user.name,
          email: user.email
        }
      })
        .then(res=> res.json())
        .then(res=>{
          if(res.success){
            console.log(chalk.green(`Secret "${secretName}" from "${scopeName}" modified successfully!`));
          }else{
            console.log(chalk.red(res.error));
          } //end if
        });
    }catch(err){
      console.log(chalk.red('Problem connecting to server.'));
      console.log(chalk.red(err));
    }
  }
};
