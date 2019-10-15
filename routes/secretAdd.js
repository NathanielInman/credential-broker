const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {name,user} = req;

  try{
    const {scopeName,secretName,secretValue} = await verify(key,req.body);

    if(!scopeName||!secretName||!secretValue){
      req.log('Secret Add (Bad Request)',true);
      return req.respond({status:400,body:{
        error: 'Missing scopeName, secretName or secretValue.'
      }})
    } //end if
    const keys = await req.broker.db.keys(),
          hasScopeAccess = user.permissions.scopes.find(s=> s.name===scopeName),
          hasScopeEditAccess = hasScopeAccess&&hasScopeAccess.value==='edit';

    // short-circuit fail-first
    if(!hasScopeEditAccess){
      req.log(`Secret Add (${scopeName})`,true);
      return req.respond({status:401,body:{
        error: `User "${name}" does not have scope edit permission.`
      }});
    }else if(!keys.includes(`scope:${scopeName}`)){
      req.log(`Secret Add (${scopeName}-NO-SCOPE)`,true);
      return req.respond({status:401,body:{
        error: `Scope "${scopeName}" does not exist.`
      }});
    }else{
      let targetScope = await req.broker.db.getItem(`scope:${scopeName}`);

      req.log(`Secret Add (${scopeName})`);
      if(!targetScope){
        targetScope = {};
        targetScope[req.body.secretName] = req.body.secretValue;
      }else{
        targetScope[req.body.secretName] = req.body.secretValue;
      } //end if
      await req.broker.db.setItem(`scope:${scopeName}`,targetScope);
      req.respond({body:{success: 'Added secret successfully.'}});
    } //end if
  }catch(err){
    req.respond({status:500,body:{error: 'Server error adding secret.'}});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
