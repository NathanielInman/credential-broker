const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');
const {verify} = require('../libraries/verify.js');
const {log} = require('../libraries/log.js');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {ip,name,user,key} = req;

  try{
    const {scopeName,secretName,secretValue} = await verify(key,req.body);

    if(!scopeName||!secretName||!secretValue){
      log(ip,name,'Secret Add (SIGNING-VERIFICATION-FAILURE)',true);
      return res.status(403).json({error: 'Request has been tempered with!'});
    } //end if
    const keys = await req.broker.db.keys(),
          hasScopeAccess = user.permissions.scopes.find(s=> s.name===scopeName),
          hasScopeEditAccess = hasScopeAccess&&hasScopeAccess.value==='edit';

    // short-circuit fail-first
    if(!hasScopeEditAccess){
      log(ip,name,`Secret Add (${scopeName})`,true);
      return res.status(401).json({
        error: `User "${name}" does not have scope edit permission.`
      });
    }else if(!keys.includes(`scope:${scopeName}`)){
      log(ip,name,`Secret Add (${scopeName}-NO-SCOPE)`,true);
      return res.status(401).json({error: `Scope "${scopeName}" does not exist.`});
    }else{
      let targetScope = await req.broker.db.getItem(`scope:${scopeName}`);

      log(ip,name,`Secret Add (${scopeName})`);
      if(!targetScope){
        targetScope = {};
        targetScope[req.body.secretName] = req.body.secretValue;
      }else{
        targetScope[req.body.secretName] = req.body.secretValue;
      } //end if
      await req.broker.db.setItem(`scope:${scopeName}`,targetScope);
      res.status(200).json({success: 'Added secret successfully.'});
    } //end if
  }catch(err){
    res.status(500).json({error: 'Server error adding secret.'})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
