const express = require('express');
const chalk = require('chalk');
const crypto = require('crypto');
const speakeasy = require('speakeasy');
const router = express.Router({mergeParams: true});
const {authSecureDecrypt} = require('../libraries/authSecure.js');
const {log} = require('../libraries/log.js');

router.post('/',express.text(),async (req,res)=>{

  // short-circuit failure
  if(!req.headers.id){
    req.log('auth2FAInitialize: Id missing',true);
    return res.status(401).send('User session id was not passed in the header.');
  }else if(!req.headers.name){
    req.log('auth2FAInitialize: Name missing',true);
    return res.status(401).send('Encrypted name missing from the header.');
  } //end if
  const session = await req.broker.db.getItem(`session:${req.headers.id}`);

  if(!session){
    req.log('auth2FAInitialize: No session to attach',true);
    return res.status(401).send('Session expired.');
  } //end if
  const {secret} = session,
        name = authSecureDecrypt(secret,req.headers.name),
        user = await req.broker.db.getItem(`user:${name}`),
        twoFactorSecret = speakeasy.generateSecret();

  req.broker.db.setItem(
    `user:${name}`,
    {...user,twoFactorSecret:twoFactorSecret.base32}
  );
  req.key = user.key;
  req.secret = secret;
  req.respond({body: {url: twoFactorSecret.otpauth_url}});
});

module.exports = {router};
