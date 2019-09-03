const fetch = require('node-fetch');
const fs = require('fs');
const chalk = require('chalk');
const {User} = require('../models/User.js');
const {prompt,confirm} = require('../libraries/prompt.js');

module.exports = {
  async scopeGetAll(){

    //short-circuit failure
    if(!fs.existsSync('./user.json')){
      return console.log(chalk.red('No user exists locally, initialize first with: ')+chalk.cyan('broker init'));
    } //end if
    const user = new User(JSON.parse(fs.readFileSync('./user.json')));

    try{
      await fetch(`${user.remoteIP}/scopeGetAll`,{
        method: 'POST',
        body: '',
        headers: {
          'Content-Type': 'application/json',
          key: fs.readFileSync(user.pgpPrivateKeyLocation).toString()
          name: user.name,
          email: user.email
        }
      })
        .then(res=> res.json())
        .then(res=>{
          if(res.success&&res.success.length){
            console.log(chalk.green(res.success));
          }else if(res.success){
            console.log(chalk.cyan('No scopes exist or you don\'t have access to view them.'));
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
