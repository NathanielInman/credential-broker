const fetch = require('node-fetch');
const fs = require('fs');
const chalk = require('chalk');
const {User} = require('../models/User.js');

module.exports = {
  async userAdd(){

    //short-circuit failure
    if(!fs.existsSync('./user.json')){
      console.log(chalk.red('No user exists locally, initialize first with: ')+chalk.cyan('broker init'));
    } //end if
    const user = new User(JSON.parse(fs.readFileSync('./user.json')));

    try{
      const data = await fetch(`${user.remoteIP}/userAdd`,{
        method: 'POST',
        body: JSON.stringify({key: fs.readFileSync(user.pgpPrivateKeyLocation).toString()}),
        headers: {
          'Content-Type': 'application/json',
          username: user.username,
          email: user.email
        }
      })
        .then(res=> res.json())
        .then(res=>{
          if(res.success){
            console.log(chalk.green(res.success));
          }else{
            console.log(chalk.red(res.error));
          } //end if
        });
    }catch(err){
      console.log('Problem connecting to server.');
      console.log(err);
    }
  }
};
