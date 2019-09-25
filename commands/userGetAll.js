const fetch = require('node-fetch');
const fs = require('fs');
const chalk = require('chalk');
const {User} = require('../models/User.js');
const {decrypt} = require('../libraries/decrypt.js');

module.exports = {
  async userGetAll(){

    // short-circuit failure
    if(!fs.existsSync('./user.json')){
      return console.log(chalk.red('No user exists locally, initialize first with: ')+chalk.cyan('broker init'));
    } //end if
    const user = new User(JSON.parse(fs.readFileSync('./user.json')));

    try{
      const data = await fetch(`${user.remoteIP}/userGetAll`,{
        method: 'POST',
        body: '',
        headers: {
          'Content-Type': 'text/plain',
          key: encodeURIComponent(fs.readFileSync('./id_rsa.pub').toString()),
          name: user.name,
          email: user.email
        }
      })
        .then(res=> res.json())
        .then(async res=>{
          if(res.success){
            const data = await decrypt(user,res.success);

            console.log(chalk.magenta('[USERS]'));
            data.forEach(userName=> console.log(chalk.green(userName)));
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
