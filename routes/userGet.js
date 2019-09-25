const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {verify} = require('../libraries/verify.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        requestedUsername = await verify(key,req.body);

  // short-circuit fail-first
  if(!requestedUsername){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(`User Get (SIGNING-VERIFICATION-FAILURE)`)
    );
    return res.status(403).json({
      error: 'Request has been tempered with!'
    });
  }else if(requestedUsername!==name&&!user.permissions.editUsers){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(`User Get (${requestedUsername})`)
    );
    return res.status(401).json({error: `User "${name}" does not have user edit permission.`});
  }else if(requestedUsername===name){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.green(`User Get (${requestedUsername})`)
    );
    return res.status(200).json({success: user});
  } //end if

  try{
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.green(`User Get (${requestedUsername})`)
    );
    const data = JSON.stringify(await req.broker.db.getItem(`user:${requestedUsername}`));

    res.status(200).json({success: await encrypt(key,data)});
  }catch(err){
    res.status(500).json({error: 'Server had a problem adding new user.'});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
