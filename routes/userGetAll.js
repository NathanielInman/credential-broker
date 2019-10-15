const express = require('express');
const chalk = require('chalk');
const router = express.Router({mergeParams: true});
const {authenticate} = require('./authenticate');

router.post('/',express.text(),authenticate,async (req,res)=>{
  const {name,user} = req,
        requestedUsername = req.body;

  try{

    // short-circuit fail-first
    if(!requestedUsername){
      req.log('Get All Users (Bad Request)',true);
      return req.respond({status:400,body:{
        error: 'Missing requestedUsername'
      }});
    }else if(!user.permissions.viewUsers){
      req.log('Get All Users',true);
      return req.respond({status:401,body:{
        error: `User "${name}" does not have user edit permission.`
      }});
    }else{
      req.log('Get All Users');
      const users = await req.broker.db.getItem('users'),
            userData = JSON.stringify(!users?[]:users.map(user=> user.name));

      req.respond({body:{success: userData}});
    } //end if
  }catch(err){
    req.respond({status:500,body:{error: 'Server error retrieving users.'}});
    console.log(chalk.red(err));
  }
});

module.exports = {router};
