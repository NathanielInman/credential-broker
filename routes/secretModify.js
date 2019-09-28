const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {verify} = require('../libraries/verify.js');
const {log} = require('../libraries/log.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        {scopeName,secretName,secretValue} = await verify(key,req.body);

  if(!scopeName||!secretName||!secretValue){
    log(ip,name,'Secret Modify (SIGNING-VERIFICATION-FAILURE)',true);
    return res.status(403).json({error: 'Request has been tempered with!'});
  } //end if
  const hasScopeAccess = user.permissions.scopes.find(s=> s.name===scopeName),
        hasScopeEditAccess = hasScopeAccess&&hasScopeAccess.value==='edit';

  try{
    const keys = await req.broker.db.keys();

    // short-circuit fail-first
    if(!hasScopeEditAccess){
      log(ip,name,`Secret Modify (${scopeName})`,true);
      return res.status(401).json({
        error: `User "${name}" does not have scope edit permission.`
      });
    }else if(!keys.includes(`scope:${scopeName}`)){
      log(ip,name,`Secret Modify (${scopeName}-NO-SCOPE)`,true);
      return res.status(401).json({error: `Scope "${scopeName}" does not exist.`});
    }else{
      let targetScope = await req.broker.db.getItem(`scope:${scopeName}`);

      log(ip,name,`Secret Modify (${scopeName})`);
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
