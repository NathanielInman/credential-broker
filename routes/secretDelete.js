const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {verify} = require('../libraries/verify.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        {scopeName,secretName} = await verify(key,req.body);

  if(!scopeName||!secretName){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(`Secret Delete (SIGNING-VERIFICATION-FAILURE)`)
    );
    return res.status(403).json({
      error: 'Request has been tempered with!'
    });
  } //end if
  const hasScopeAccess = user.permissions.scopes.find(s=> s.name===scopeName),
        hasScopeEditAccess = hasScopeAccess&&hasScopeAccess.value==='edit';

  try{
    const targetScope = await req.broker.db.getItem(`scope:${scopeName}`);

    // short-circuit fail-first
    if(!hasScopeEditAccess){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`Secret Delete (${scopeName})`)
      );
      return res.status(401).json({
        error: `User "${name}" does not have scope edit permission.`
      });
    }else if(!targetScope){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`Secret Delete (${scopeName}-NO-SCOPE)`)
      );
      return res.status(401).json({
        error: `Scope "${scopeName}" does not exist.`
      });
    }else{
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.green(`Secret Delete (${scopeName})`)
      );
      delete targetScope[secretName];
      await req.broker.db.setItem(`scope:${scopeName}`);
      res.status(200).json({success: 'Deleted scope secret successfully.'});
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error modifying user.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
