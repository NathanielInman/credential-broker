const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',authenticate,async (req,res)=>{
  const {ip,name} = req;

  // short-circuit fail-first
  if(!req.user.permissions.createScopes){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(': ')+
      chalk.red('[FAILURE] ')+
      chalk.green(` Add Scope (${req.body.scopeName})`)
    );
    return res.status(401).json({
      error: `User "${user.name}" does not have scope create permission.`
    });
  } //end if

  try{
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${name}>`)+
      chalk.grey(':')+
      chalk.green(` Add Scope (${req.body.scopeName})`)
    );
    await req.broker.db.setItem(`scope:${req.body.scopeName}`,{});
    let scopes = await req.broker.db.getItem('scopes');

    if(scopes&&scopes.find(s=>s.name===req.body.scopeName)){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${name}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(`Add Scope (${req.body.scopeName})`)
      );
      return res.status(400).json({
        error: `Scope name "${req.body.scopeName}" already exists and cannot be created.`
      });
    }else if(!scopes){
      scopes = [{name: req.body.scopeName,publicKey: req.body.scopePublicKey}];
    }else{
      scopes.push({name: req.body.scopeName,publicKey: req.body.scopePublicKey});
    } //end if
    await req.broker.db.setItem('scopes',scopes);
    req.user.permissions.scopes.push({name: req.body.scopeName,value: 'edit'});
    await req.broker.db.setItem(`user:${req.user.name}`,req.user);
    res.status(200).json({success: `Added scope ${req.body.scopeName}`});
  }catch(err){
    res.status(500).json({error: 'Server had a problem adding new scope.'});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
