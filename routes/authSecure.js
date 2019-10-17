const express = require('express');
const chalk = require('chalk');
const crypto = require('crypto');
const router = express.Router({mergeParams: true});
const {log} = require('../libraries/log.js');

router.post('/',async (req,res)=>{

  // short-circuit failure
  if(!req.headers.key){
    req.log('authSecure: Key missing',true);
    return res.status(401).send('User key was not passed in the header.');
  }else if(!req.headers.id){
    req.log('authSecure: Id missing',true);
    return res.status(401).send('User session id was not passed in the header.');
  }else if(!req.headers.prime){
    req.log('authSecure: Prime missing',true);
    return res.status(401).send('Diffie hellman prime missing in the header.');
  } //end if

  const {id,key,prime} = req.headers;

  console.log('computing secret',id,key,prime);
  const server = crypto.createDiffieHellman(prime,'base64'),
        serverKey = server.generateKeys('base64'),
        secret  = server.computeSecret(key,'base64','base64'),
        ip = req.headers['x-forwarded-for']||req.connection.remoteAddress;

  log(ip,id,'Auth Secure');
  console.log('attempting to create session',id);
  await req.broker.db.setItem(`session:${id}`,{secret,authenticated:false});
  res.status(200).json(serverKey);
});

module.exports = {router};
