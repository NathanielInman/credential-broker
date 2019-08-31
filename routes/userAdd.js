const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',authenticate,async (req,res)=>{
  const ip = req.headers['x-forwarded-for'] ||
          req.connection.remoteAddress,
        username = req.headers.username;

  // short-circuit fail-first
  if(!req.user.permissions.editUsers){
    console.log(
      chalk.cyan(`[${ip}]`)+
      chalk.magenta(`<${username}>`)+
      chalk.grey(':')+
      chalk.green(` Add User (${req.body.username})`)
    );
    return res.status(401).json({error: `User "${user.username}" does not have user edit permission.`});
  } //end if

  try{
    await req.broker.db.setitem(
      `user:${req.body.username}`,
      {
        date: (new Date).toISOString(),
        username: req.body.username,
        addedBy: username,
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
  }catch(err){
    res.status(500).json({error: 'Server had a problem adding new user.'});
    console.log(chalk.red(err));
  }
  res.status(200).json({success: 'Added user'});
});

module.exports = {router};
