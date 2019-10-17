const express = require('express');
const chalk = require('chalk');
const {authSecureDecrypt} = require('../libraries/authSecure.js');
const {verify} = require('../libraries/verify.js');

module.exports = {
  async authenticate(req,res,next){
    const {ip} = req;

    // short-circuit failure
    if(!req.headers.id){
      req.log(`${req.originalUrl}: Key missing`,true);
      return res.status(401).send('User key was not passed in the header.');
    } //end if

    // short-circuit failure
    if(!req.headers.name){
      req.log(`${req.originalUrl}: Username missing`,true);
      return res.status(401).send('User name was not passed in the header.');
    } //end if

    try{
      const secret = await req.broker.db.getItem(`session:${req.headers.id}`),
            name = authSecureDecrypt(secret,req.headers.name),
            email = authSecureDecrypt(secret,req.headers.email);

      req.secret = secret; //diffie-hellman key to encrypt transmission payloads
      req.name = name;
      req.email = email;
    }catch(err){
      console.log(err);
      req.log(`${req.originalUrl}: Authentication Failure`,true);
      return res.status(401).send('Improper authentication headers.');
    }

    const user = await req.broker.db.getItem(`user:${req.name}`);

    //short-circuit failure
    if(!user){
      req.log(`${req.originalUrl}: User does not exist`,true);
      return res.status(401).send('User does not exist.');
    }else{

      // attach the user to the request so any route can attach their
      // individual requirements or filters based on their permissions
      req.user = user;
      req.key = user.key; //public key used to encrypt payload
    } //end if

    //short-circuit failure
    if(!user.permissions){
      req.log(`${req.originalUrl}: User data is corrupted`,true);
      return req.respond({
        status:401,body:{error: 'Your user structure invalid, data corrupted.'}
      });
    } //end if

    //a body was sent with the request, verify the identity of the body
    //matches the users public key; otherwise the request was tampered with
    //or someone is spoofing or impersonating the user
    if(req.body){
      try{
        req.body = await verify(user.key,req.body);
      }catch(err){
        console.log(err);
        req.log(`${req.originalUrl}: Signing verification failure`,true);
        return req.respond({
          status:401,body:{error: 'Request has been tampered with!'}
        });
      }
    } //end if

    // short-circuit failure
    if(!user.session||!user.session.authenticated){

      // we omit logging this, it's an expected thing to have to re-auth
      // a session
      return req.respond({status:401,body:{error: 'Session expired.'}});
    } //end if
    next();
  }
};
