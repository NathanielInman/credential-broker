const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {name,user} = req,
        {scopeName,secretName} = req.body;

  if(!scopeName||!secretName){
    req.log('Secret Delete (Bad Request)',true);
    return req.respond({status:400,body:{
      error: 'Missing scopeName or secretName'
    }});
  } //end if
  const hasScopeAccess = user.permissions.scopes.find(s=> s.name===scopeName),
        hasScopeEditAccess = hasScopeAccess&&hasScopeAccess.value==='edit';

  try{
    const targetScope = await req.broker.db.getItem(`scope:${scopeName}`);

    // short-circuit fail-first
    if(!hasScopeEditAccess){
      req.log(`Secret Delete (${scopeName})`,true);
      return req.respond({status:401,body:{
        error: `User "${name}" does not have scope edit permission.`
      }});
    }else if(!targetScope){
      req.log(`Secret Delete (${scopeName}-NO-SCOPE)`,true);
      return req.respond({status:401,body:{
        error: `Scope "${scopeName}" does not exist.`
      }});
    }else{
      req.log(`Secret Delete (${scopeName})`);
      delete targetScope[secretName];
      await req.broker.db.setItem(`scope:${scopeName}`);
      req.respond({body:{success: 'Deleted scope secret successfully.'}});
    } //end if
  }catch(err){
    req.respond({status:500,body:{error: 'Server error modifying user.'}});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
