const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {verify} = require('../libraries/verify.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        {scopeName,secretName,secretValue} = await verify(key,req.body);

  if(!scopeName||!secretName||!secretValue){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(`Secret Modify (SIGNING-VERIFICATION-FAILURE)`)
    );
    return res.status(403).json({
      error: 'Request has been tempered with!'
    });
  } //end if
  const hasScopeAccess = user.permissions.scopes.find(s=> s.name===scopeName),
        hasScopeEditAccess = hasScopeAccess&&hasScopeAccess.value==='edit';

  try{
    const keys = await req.broker.db.keys();

    // short-circuit fail-first
    if(!hasScopeEditAccess){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`Secret Modify (${scopeName})`)
      );
      return res.status(401).json({
        error: `User "${name}" does not have scope edit permission.`
      });
    }else if(!keys.includes(`scope:${scopeName}`)){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`Secret Modify (${scopeName}-NO-SCOPE)`)
      );
      return res.status(401).json({
        error: `Scope "${scopeName}" does not exist.`
      });
    }else{
      let targetScope = await req.broker.db.getItem(`scope:${scopeName}`);

      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.green(`Secret Modify (${scopeName})`)
      );
      if(!targetScope){
        targetScope = {};
        targetScope[secretName] = secretValue;
      }else{
        targetScope[secretName] = req.body.secretValue;
      } //end if
      await req.broker.db.setItem(`scope:${scopeName}`,targetScope);
      res.status(200).json({success: 'Modified secret successfully.'});
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error adding secret.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
