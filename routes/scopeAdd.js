const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {name,user} = req,
        {scopeName,scopePublicKey} = req.body;

  // short-circuit fail-first
  if(!scopeName||!scopePublicKey){
    req.log('Add Scope (Bad Request)',true);
    return req.respond({status:400,body: {
      error: 'Missing scopePublicKey or scopeName'
    }});
  }else if(!user.permissions.createScopes){
    req.log(`Add Scope (${scopeName})`,true);
    return respond({
      status:401,body:{error: `User ${name}" does not have scope create permission.`}
    });
  } //end if

  try{
    req.log(`Add Scope (${scopeName})`);
    await req.broker.db.setItem(`scope:${scopeName}`,{});
    let scopes = await req.broker.db.getItem('scopes');

    if(scopes&&scopes.find(s=>s.name===scopeName)){
      req.log(`Add Scope (${scopeName})`,true);
      return req.respond({status:400,body:{
        error: `Scope name "${scopeName}" already exists and cannot be created.`
      }});
    }else if(!scopes){
      scopes = [{name: scopeName,publicKey: scopePublicKey}];
    }else{
      scopes.push({name: scopeName,publicKey: scopePublicKey});
    } //end if
    await req.broker.db.setItem('scopes',scopes);
    user.permissions.scopes.push({name: scopeName,value: 'edit'});
    await req.broker.db.setItem(`user:${name}`,user);
    req.respond({body:{success: `Added scope ${scopeName}`}});
  }catch(err){
    req.respond({status:500,body:{error: 'Server had a problem adding new scope.'}});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
