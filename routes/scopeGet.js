const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {encrypt} = require('../libraries/encrypt.js');
const {verify} = require('../libraries/verify.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        {scopeName} = await verify(key,req.body);

  if(!scopeName){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(`Get Scope (SIGNING-VERIFICATION-FAILURE)`)
    );
    return res.status(403).json({
      error: 'Request has been tempered with!'
    });
  } //end if
  const hasScopeAccess = req.user.permissions.scopes.find(s=> s.name===scopeName);

  // short-circuit fail-first
  if(scopeName!==name&&!hasScopeAccess){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(`Get Scope (${scopeName})`)
    );
    return res.status(401).json({error: `User "${name}" does not have get all scopes permission.`});
  }else if(scopeName===name){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.green(`Get Scope (${scopeName}-IS-SCOPE)`)
    );
    return res.status(200).json({success: user});
  } //end if

  try{
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.green(`Get Scope (${scopeName})`)
    );
    const data = (await req.broker.db.getItem(`scope:${scopeName}`)||{});

    res.status(200).json({success: await encrypt(key,data)});
  }catch(err){
    res.status(500).json({error: 'Server had a problem getting scope.'});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
