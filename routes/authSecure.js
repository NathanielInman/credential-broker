const express = require('express');
const chalk = require('chalk');
const crypto = require('crypto');
const router = express.Router({mergeParams: true});
const {log} = require('../libraries/log.js');

router.post('/',async (req,res)=>{
  const {id,key,prime} = req.headers,
        server = crypto.createDiffieHellman(prime,'base64'),
        serverKey = server.generateKeys('base64'),
        secret  = server.computeSecret(key,'base64','base64'),
        ip = req.headers['x-forwarded-for']||req.connection.remoteAddress;

  log(ip,id,'Auth Secure');
  await req.broker.db.setItem(`session:${id}`,secret);
  res.status(200).json(serverKey);
});

module.exports = {router};
