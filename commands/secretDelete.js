const fetch = require('node-fetch');
const fs = require('fs');
const chalk = require('chalk');
const {User} = require('../models/User.js');
const {prompt,confirm} = require('../libraries/prompt.js');

module.exports = {
  async secretDelete(scopeName,secretName){

    // short-circuit failure
    if(!fs.existsSync('./user.json')){
      return console.log(chalk.red('No user exists locally, initialize first with: ')+chalk.cyan('broker init'));
    } //end if
    const test = await confirm(chalk.green(`Delete "${secretName}" from scope "${scopeName}"?`));

    if(!test) return;
    const user = new User(JSON.parse(fs.readFileSync('./user.json')));

    try{
      const data = await fetch(`${user.remoteIP}/secretDelete`,{
              method: 'POST',
              body: JSON.stringify({
                name: user.name,
                scopeName,
                secretName
              }),
              headers: {
                'Content-Type': 'application/json',
                key: encodeURIComponent(fs.readFileSync('./id_rsa.pub').toString()),
                name: user.name,
                email: user.email
              }
            })
            .then(res=> res.json())
            .then(res=>{
              if(res.success){
                console.log(chalk.green(`Secret "${secretName}" from "${scopeName}" deleted successfully!`));
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
