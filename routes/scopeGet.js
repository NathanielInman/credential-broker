const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {encrypt} = require('../libraries/encrypt.js');
const {verify} = require('../libraries/verify.js');
const {log} = require('../libraries/log.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        {scopeName} = await verify(key,req.body);

  if(!scopeName){
    log(ip,name,'Get Scope (SIGNING-VERIFICATION-FAILURE)',true);
    return res.status(403).json({error: 'Request has been tempered with!'});
  } //end if
  const hasScopeAccess = req.user.permissions.scopes.find(s=> s.name===scopeName);

  // short-circuit fail-first
  if(scopeName!==name&&!hasScopeAccess){
    log(ip,name,`Get Scope (${scopeName})`,true);
    return res.status(401).json({error: `User "${name}" does not have get all scopes permission.`});
  }else if(scopeName===name){
    log(ip,name,`Get Scope (${scopeName}-IS-SCOPE)`);
    return res.status(200).json({success: user});
  } //end if

  try{
    log(ip,name,`Get Scope (${scopeName})`);
    const data = (await req.broker.db.getItem(`scope:${scopeName}`)||{});

    res.status(200).json({success: await encrypt(key,data)});
  }catch(err){
    res.status(500).json({error: 'Server had a problem getting scope.'});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
