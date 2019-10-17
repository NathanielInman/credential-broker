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
  const secret = await req.broker.db.getItem(`session:${req.headers.id}`),
        name = authSecureDecrypt(secret,req.headers.name),
        user = await req.broker.db.getItem(`user:${name}`),
        randomNumber = crypto.randomBytes(256).toString('base64')

  req.broker.db.setItem(
    `user:${name}`,
    {
      ...user,
      session:{
        id:req.headers.id,challenge:randomNumber
      }
    }
  );
  req.key = user.key;
  req.secret = secret;
  req.respond({body: randomNumber});
});

module.exports = {router};
