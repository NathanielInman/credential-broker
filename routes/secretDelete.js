const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {verify} = require('../libraries/verify.js');
const {log} = require('../libraries/log.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req,
        {scopeName,secretName} = await verify(key,req.body);

  if(!scopeName||!secretName){
    log(ip,name,'Secret Delete (SIGNING-VERIFICATION-FAILURE)',true);
    return res.status(403).json({error: 'Request has been tempered with!'});
  } //end if
  const hasScopeAccess = user.permissions.scopes.find(s=> s.name===scopeName),
        hasScopeEditAccess = hasScopeAccess&&hasScopeAccess.value==='edit';

  try{
    const targetScope = await req.broker.db.getItem(`scope:${scopeName}`);

    // short-circuit fail-first
    if(!hasScopeEditAccess){
      log(ip,name,`Secret Delete (${scopeName})`,true);
      return res.status(401).json({
        error: `User "${name}" does not have scope edit permission.`
      });
    }else if(!targetScope){
      log(ip,name,`Secret Delete (${scopeName}-NO-SCOPE)`,true);
      return res.status(401).json({
        error: `Scope "${scopeName}" does not exist.`
      });
    }else{
      log(ip,name,`Secret Delete (${scopeName})`);
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
