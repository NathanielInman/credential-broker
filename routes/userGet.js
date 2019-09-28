const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {verify} = require('../libraries/verify.js');
const {log} = require('../libraries/log.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        requestedUsername = await verify(key,req.body);

  // short-circuit fail-first
  if(!requestedUsername){
    log(ip,name,'User Get (SIGNING-VERIFICATION-FAILURE)',true);
    return res.status(403).json({error: 'Request has been tempered with!'});
  }else if(requestedUsername!==name&&!user.permissions.editUsers){
    log(ip,name,`User Get (${requestedUsername})`,true);
    return res.status(401).json({error: `User "${name}" does not have user edit permission.`});
  }else if(requestedUsername===name){
    log(ip,name,`User Get (${requestedUsername})`);
    return res.status(200).json({success: user});
  } //end if

  try{
    log(ip,name,`User Get (${requestedUsername})`);
    const data = JSON.stringify(await req.broker.db.getItem(`user:${requestedUsername}`));

    res.status(200).json({success: await encrypt(key,data)});
  }catch(err){
    res.status(500).json({error: 'Server had a problem adding new user.'});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
