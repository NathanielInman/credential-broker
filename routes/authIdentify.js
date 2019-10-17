const express = require('express');
const chalk = require('chalk');
const crypto = require('crypto');
const router = express.Router({mergeParams: true});
const {authSecureDecrypt} = require('../libraries/authSecure.js');
const {log} = require('../libraries/log.js');

router.post('/',express.text(),async (req,res)=>{

  // short-circuit failure
  if(!req.headers.id){
    req.log('authIdentify: Id missing',true);
    return res.status(401).send('User session id was not passed in the header.');
  }else if(!req.headers.name){
    req.log('authIdentify: Name missing',true);
    return res.status(401).send('Encrypted name missing from the header.');
  } //end if
  const session = await req.broker.db.getItem(`session:${req.headers.id}`);

  console.log('session',session);
  if(!session){
    req.log('authIdentify: No session to attach',true);
    return res.status(401).send('Session expired.');
  } //end if
  const {secret} = session,
        name = authSecureDecrypt(secret,req.headers.name),
        user = await req.broker.db.getItem(`user:${name}`),
        randomNumber = crypto.randomBytes(256).toString('base64')

  req.broker.db.setItem(`session:${req.headers.id}`,{secret,challenge:randomNumber});
  req.key = user.key;
  req.secret = secret;
  req.respond({body: randomNumber});
});

module.exports = {router};
