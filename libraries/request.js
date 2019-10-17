const fs = require('fs');
const fetch = require('node-fetch');
const chalk = require('chalk');
const crypto = require('crypto');
const readline = require('readline');
const {User} = require('../models/User.js');
const {spinner} = require('./spinner.js');
const {decrypt,sign} = require('./authPGP.js');
const {authSecure,authSecureEncrypt,authSecureDecrypt} = require('./authSecure.js');

async function authenticate(user){
  await authSecure(user);
  return fetch(`${user.remoteIP}/authIdentify`,{
    method: 'POST',body:'',
    headers: {
      'Content-Type': 'text/plain',
      id: crypto.createHash('md5').update(user.name).digest('hex'),
      name: authSecureEncrypt(user.secret,user.name),
      email: authSecureEncrypt(user.secret,user.email)
    }
  })
    .then(res=> res.text())
    .then(res=> authSecureDecrypt(user.secret,res))
    .then(async res=> JSON.parse(await decrypt(user,res)))
    .then(challenge=>{
      return fetch(`${user.remoteIP}/authChallenge`,{
        method: 'POST',body:'',
        headers: {
          'Content-Type': 'text/plain',
          id: crypto.createHash('md5').update(user.name).digest('hex'),
          name: authSecureEncrypt(user.secret,user.name),
          email: authSecureEncrypt(user.secret,user.email),
          challenge: authSecureEncrypt(
            user.secret,
            crypto.createHash('md5').update(challenge).digest('hex')
          )
        }
      })
    })
    .then(res=> res.text())
    .then(res=> authSecureDecrypt(user.secret,res))
    .then(async res=> JSON.parse(await decrypt(user,res)));
} //end authenticate()

function request({user,uri,body='',method='POST'}={}){
  return fetch(`${user.remoteIP}/${uri}`,{
    method,body,
    headers: {
      'Content-Type': 'text/plain',
      id: crypto.createHash('md5').update(user.name).digest('hex'),
      name: authSecureEncrypt(user.secret,user.name),
      email: authSecureEncrypt(user.secret,user.email)
    }
  });
} //end request()

module.exports = {
  async request(uri,body='',method='POST'){
    spinner.setSpinnerTitle(chalk.yellow('Synchonizing with server... %s'));
    spinner.start();

    //short-circuit failure
    if(!fs.existsSync('./user.json')){
      throw new Error('No user exists locally, initialize first with "broker init"');
    } //end if
    const user = new User(JSON.parse(fs.readFileSync('./user.json')));

    if(body!=='') body=await sign(user,JSON.stringify(body));
    return request({user,uri,method,body})
      .then(res=> res.text())
      .then(async res=>{

        if(res==='Session expired.'){
          await authenticate(user);
          return request({user,uri,method,body});
        } //end if
        return res;
      })
      .then(res=> authSecureDecrypt(user.secret,res))
      .then(async res=> ({user,...JSON.parse(await decrypt(user,res))}))
      .then(res=>{
        spinner.stop();
        readline.cursorTo(process.stdout, 0);
        console.log(chalk.green('Synchronizing with server... (done)'));
        return res;
      });
  }
};
