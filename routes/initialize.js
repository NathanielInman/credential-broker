const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {log} = require('../libraries/log.js');
const {mail} = require('../libraries/mail.js');

router.post('/',express.json(),async (req,res)=>{
  const {name,email,ip,key} = req,
        user = await req.broker.db.getItem(`user:${name}`);

  // short-circuit success-first
  if(user){
    log(ip,name,'Initialize User');
    return res.status(200).json({success: 'You user is properly linked'});
  } //end if
  const users = await req.broker.db.getItem('users'),
        scopes = await req.broker.db.getItem('scopes');

  if(!scopes&&!users){
    let newUser = {
      date: (new Date).toISOString(),
      name,email,key,
      addedBy: 'ROOT',
      addedByIP: ip,
      permissions: {
        viewUsers: req.body.permissions.viewUsers,
        editUsers: req.body.permissions.editUsers,
        viewScopeNames: req.body.permissions.viewScopeNames,
        createScopes: req.body.permissions.createScopes,
        scopes: req.body.permissions.scopes.map(s=> ({name: s.name,value: s.value}))
      }
    };

    log(ip,name,'Initialize User (FIRST)');
    await req.broker.db.setItem(`user:${name}`,newUser);
    await req.broker.db.setItem('users',[newUser]);
    res.status(200).json({success: 'First user created successfully with requested permissions.'});
  }else if(!users){
    const {value} = req.broker.strategies.find(s=> s.name==='First account gets access');

    if(value){
      log(ip,name,'Initialize User (FIRST-GAINED-ALL-ACCESS)');
      let newUser = {
        date: (new Date).toISOString(),
        name,email,key,
        addedBy: 'ROOT',
        addedByIP: ip,
        permissions: {
          viewUsers: req.body.permissions.viewUsers,
          editUsers: req.body.permissions.editUsers,
          viewScopeNames: req.body.permissions.viewScopeNames,
          createScopes: req.body.permissions.createScopes,
          scopes: req.body.permissions.scopes.map(s=> ({name: s.name,value: s.value}))
        }
      };

      await req.broker.db.setItem(`user:${name}`,newUser);
      await req.broker.db.setItem('users',[newUser]);
      res.status(200).json({success: `First user created successfully and given access to scopes: ${scopes.map(s=>s.name).join()}.`});
    }else{
      let newUser = {
        date: (new Date).toISOString(),
        name,email,key,
        addedBy: 'ROOT',
        addedByIP: ip,
        permissions: {
          viewUsers: false,
          editUsers: false,
          viewScopeNames: false,
          createScopes: false,
          scopes: req.body.permissions.scopes.map(s=> ({name: s.name,value: false}))
        }
      };

      await req.broker.db.setItem(`user:${name}`,newUser);
      await req.broker.db.setItem('users',[newUser]);
      log(ip,name,'Initialize User (FIRST-ALL-ACCESS-DENIED)',true);
      res.status(200).json({success: 'User created but without any permissions because "First account strategy get access" strategy turned off.'});
    } //end if
  }else{
    const names = users
      .filter(user=> user.permissions.editUsers)
      .map(user=> user.name);

    log(ip,name,'Initialize User (NOT-SETUP)',true);
    res.status(400).json({error: `In order to be added as a user, please contact user administrators: ${names.join()}`});
  } //end if
});

module.exports = {router};
