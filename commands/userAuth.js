const chalk = require('chalk');
const qrcode = require('qrcode-terminal');
const {prompt,confirm} = require('../libraries/prompt.js');
const {request} = require('../libraries/request.js');

module.exports = {
  async userAuth(){
    try{
      const {url} = await request('auth2FAInitialize');

      qrcode.generate(url,{small:true});
      let bool,
          userToken = await prompt(chalk.green('Enter two-factor token: '));

      do{
        bool = await confirm(chalk.green(`Is this correct: "${userToken}"?`));
        if(!bool){
          console.log(chalk.green('No problem, let\'s try again.'));
          userToken = await prompt(chalk.green('Enter two-factor token: '));
        } //end if
      }while(!bool)
      const {valid} = await request('auth2FAValidate',userToken);

      if(valid){
        console.log(chalk.green('Two-factor successfully setup.'));
      }else{
        console.log(chalk.red('Two-factor code was not accurate. Please retry setting up.'));
      } //end if
    }catch(err){
      console.log(chalk.red('Problem connecting to server.'));
      console.log(chalk.red(err));
    }
  }
};
