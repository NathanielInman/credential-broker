const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});

router.post('/',async (req,res)=>{
  const {username,ip} = req,
        user = await req.broker.db.getItem(`user:${username}`);

  // short-circuit success-first
  if(user){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${username}>`)+
      chalk.grey(':')+
      chalk.green(' Initialize User')
    );
    return res.status(200).json({success: 'You user is properly linked'});
  } //end if
  const users = await req.broker.db.getItem('users'),
        scopes = await req.broker.db.getItem('scopes');

  if(!scopes&&!users){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${username}>`)+
      chalk.grey(':')+
      chalk.green(' Initialize User (FIRST)')
    );
    await req.broker.db.setItem(
      `user:${username}`,
      {
        date: (new Date).toISOString(),
        username: req.body.username,
        addedBy: 'ROOT',
        addedByIP: ip,
        key: req.body.key,
        permissions: {
          viewUsers: req.body.permissions.viewUsers,
          editUsers: req.body.permissions.editUsers,
          viewScopeNames: req.body.permissions.viewScopeNames,
          createScopes: req.body.permissions.createScopes,
          scopes: req.body.permissions.scopes.map(s=> ({name: s.name,value: s.value}))
        }
      }
    );
    res.status(200).json({success: 'First user created successfully with requested permissions.'});
  }else if(!users){
    const {value} = req.broker.strategies.find(s=> s.name==='First account gets access');

    if(value){
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${username}>`)+
        chalk.grey(':')+
        chalk.green(' Initialize User (FIRST-GAINED-ALL-ACCESS)')
      );
      await req.broker.db.setItem(
        `user:${username}`,
        {
          date: (new Date).toISOString(),
          ame: req.body.name,
          addedBy: 'ROOT',
          addedByIP: ip,
          key: req.body.key,
          permissions: {
            viewUsers: req.body.permissions.viewUsers,
            editUsers: req.body.permissions.editUsers,
            viewScopeNames: req.body.permissions.viewScopeNames,
            createScopes: req.body.permissions.createScopes,
            scopes: req.body.permissions.scopes.map(s=> ({name: s.name,value: s.value}))
          }
        }
      );
      res.status(200).json({success: `First user created successfully and given access to scopes: ${scopes.map(s=>s.name).join()}.`});
    }else{
      await req.broker.db.setItem(
        `user:${username}`,
        {
          date: (new Date).toISOString(),
          name: req.body.name,
          addedBy: 'ROOT',
          addedByIP: ip,
          key: req.body.key,
          permissions: {
            viewUsers: false,
            editUsers: false,
            viewScopeNames: false,
            createScopes: false,
            scopes: req.body.permissions.scopes.map(s=> ({name: s.name,value: false}))
          }
        }
      );
      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${username}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(' Initialize User (FIRST-ALL-ACCESS-DENIED)')
      );
      res.status(200).json({success: 'User created but without any permissions because "First account strategy get access" strategy turned off.'});
    } //end if
  }else{
    const usernames = users
      .filter(user=> user.permissions.editUsers)
      .map(user=> user.name);

      console.log(
        chalk.cyan(`[${ip}]`)+
        chalk.magenta(`<${username}>`)+
        chalk.grey(': ')+
        chalk.red('[FAILURE] ')+
        chalk.green(' Initialize User (NOT-SETUP)')
      );
    res.status(400).json({error: `In order to be added as a user, please contact user administrators: ${usernames.join()}`});
  } //end if
});

module.exports = {router};
