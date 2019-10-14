const express = require('express');
const chalk = require('chalk');
const {authSecureDecrypt} = require('../libraries/authSecure.js');
const {log} = require('../libraries/log.js');

module.exports = {
  async authenticate(req,res,next){
    const {ip,key} = req;

    // short-circuit failure
    if(!req.headers.key){
      log(ip,'unknown',`${req.originalUrl}: Key missing`,true);
      return res.status(402).json({error: 'Your key was not passed in the header.'});
    } //end if

    // short-circuit failure
    if(!req.headers.name){
      log(ip,'unknown',`${req.originalUrl}: Username missing`,true);
      return res.status(401).json({error: 'Your name was not passed in the header.'});
    } //end if

    try{
      const secret = await req.broker.db.getItem(`session:${req.headers.key}`),
            name = authSecureDecrypt(secret,req.headers.name),
            email = authSecureDecrypt(secret,req.headers.email);

      req.secret = secret; //diffie-hellman key to encrypt transmission payloads
      req.name = name;
      req.email = email;
    }catch(err){
      console.log(err);
      log(ip,'unknown',`${req.originalUrl}: Authentication Faiulre`,true);
      return res.status(401).json({error: 'Improper authentication headers.'});
    }

    const user = await req.broker.db.getItem(`user:${req.name}`);

    //short-circuit failure
    if(!user){
      log(ip,req.name,`${req.originalUrl}: User does not exist`,true);
      return res.status(401).json({error: 'Your user does not exist.'});
    }else{

      // attach the user to the request so any route can attach their
      // individual requirements or filters based on their permissions
      req.user = user;
      req.key = user.key; //public key used to encrypt payload
    } //end if

    //short-circuit failure
    if(!user.permissions){
      log(ip,req,name,`${req.originalUrl}: User data is corrupted`,true);
      return res.status(401).json({error: 'Your user structure invalid, data corrupted.'});
    } //end if
    next();
  }
};
