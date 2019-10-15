const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {name,user} = req,
        requestedUsername = req.body;

  // short-circuit fail-first
  if(!requestedUsername){
    req.log('User Get (Bad Request)',true);
    return req.respond({status:400,body:{
      error: 'Missing requestedUsername'
    }})
  }else if(requestedUsername!==name&&!user.permissions.editUsers){
    req.log(`User Get (${requestedUsername})`,true);
    return req.respond({status:401,body:{
      error: `User "${name}" does not have user edit permission.`
    }});
  }else if(requestedUsername===name){
    req.log(`User Get (${requestedUsername})`);
    return req.respond({body:{success: user}});
  } //end if

  try{
    req.log(`User Get (${requestedUsername})`);
    const data = JSON.stringify(await req.broker.db.getItem(`user:${requestedUsername}`));

    req.respond({body:{success: data}});
  }catch(err){
    req.respond({status:500,body:{error: 'Server had a problem adding new user.'}});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
