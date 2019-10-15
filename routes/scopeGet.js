const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {name,user} = req,
        {scopeName} = req.body;

  if(!scopeName){
    req.log('Get Scope (Bad Request)',true);
    return req.respond({status:400,body:{error: 'Missing scopeName'}});
  } //end if
  const hasScopeAccess = req.user.permissions.scopes.find(s=> s.name===scopeName);

  // short-circuit fail-first
  if(scopeName!==name&&!hasScopeAccess){
    req.log(`Get Scope (${scopeName})`,true);
    return req.respond({status:401,body:{
      error: `User "${name}" does not have get all scopes permission.`
    }})
  }else if(scopeName===name){
    req.log(`Get Scope (${scopeName}-IS-SCOPE)`);
    return req.respond({body:{success: user}});
  } //end if

  try{
    req.log(`Get Scope (${scopeName})`);
    const data = (await req.broker.db.getItem(`scope:${scopeName}`)||{});

    req.respond({body:{success: data}});
  }catch(err){
    req.respond({status:500,body:{error: 'Server had a problem getting scope.'}});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
