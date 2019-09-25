const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {verify} = require('../libraries/verify.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        {scopeName,scopePublicKey} = await verify(key,req.body);

  // short-circuit fail-first
  if(!scopeName||!scopePublicKey){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(`Add Scope (SIGNING-VERIFICATION-FAILURE)`)
    );
    return res.status(403).json({
      error: 'Request has been tempered with!'
    });
  }else if(!user.permissions.createScopes){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(`Add Scope (${scopeName})`)
    );
    return res.status(401).json({
      error: `User "${name}" does not have scope create permission.`
    });
  } //end if

  try{
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(':')+
      chalk.green(` Add Scope (${scopeName})`)
    );
    await req.broker.db.setItem(`scope:${scopeName}`,{});
    let scopes = await req.broker.db.getItem('scopes');

    if(scopes&&scopes.find(s=>s.name===scopeName)){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`Add Scope (${scopeName})`)
      );
      return res.status(400).json({
        error: `Scope name "${scopeName}" already exists and cannot be created.`
      });
    }else if(!scopes){
      scopes = [{name: scopeName,publicKey: scopePublicKey}];
    }else{
      scopes.push({name: scopeName,publicKey: scopePublicKey});
    } //end if
    await req.broker.db.setItem('scopes',scopes);
    user.permissions.scopes.push({name: scopeName,value: 'edit'});
    await req.broker.db.setItem(`user:${name}`,user);
    res.status(200).json({success: `Added scope ${scopeName}`});
  }catch(err){
    res.status(500).json({error: 'Server had a problem adding new scope.'});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
