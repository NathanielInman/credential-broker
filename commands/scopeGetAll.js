const fetch = require('node-fetch');
const openpgp = require('openpgp');
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
          key: encodeURIComponent(fs.readFileSync('./id_rsa.pub').toString()),
          name: user.name,
          email: user.email
        }
      })
        .then(res=> res.json())
        .then(async res=>{
          if(res.success&&res.success.length){
            const privateKey = fs.readFileSync('./id_rsa').toString(),
                  privateKeys = (await openpgp.key.readArmored(privateKey)).keys;

            await Promise.all(privateKeys.map(k=> k.decrypt(user.email)));
            const {data} = await openpgp.decrypt({
              message: await openpgp.message.readArmored(res.success),
              privateKeys
            });

            console.log(chalk.green(data));
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
