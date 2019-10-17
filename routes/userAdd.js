const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {name,user} = req,
        newUser = req.body

  if(!newUser){
    req.log('User Add (Bad Request)',true);
    return req.respond({status:400,body:{
      error: 'Missing user object'
    }});
  } //end if

  // short-circuit fail-first
  if(!user.permissions.editUsers){
    req.log(`Add User (${newUser.name})`,true);
    return req.respond({status:401,body:{
      error: `User "${name}" does not have user edit permission.`
    }});
  } //end if

  try{
    req.log(`Add User (${newUser.name})`);
    let newUser = {
      date: (new Date).toISOString(),
      name: newUser.name,
      email: newUser.email,
      addedBy: name,
      addedByIP: ip,
      key: newUser.key,
      permissions: {
        viewUsers: newUser.permissions.viewUsers,
        editUsers: newUser.permissions.editUsers,
        viewScopeNames: newUser.permissions.viewScopeNames,
        createScopes: newUser.permissions.createScopes,
        scopes: newUser.permissions.scopes.map(s=> ({name: s.name,value: s.value}))
      }
    };

    const users = await req.broker.db.getItem('users');

    await req.broker.db.setItem(`user:${newUser.name}`,newUser);
    users.push({name: newUser.name});
    await req.broker.db.setItem('users',users);
    req.respond({body:{success: `Added user ${newUser.name}`}});
  }catch(err){
    req.respond({status:500,body:{error: 'Server had a problem adding new user.'}})
    console.log(chalk.red(err));
  }
});

module.exports = {router};
