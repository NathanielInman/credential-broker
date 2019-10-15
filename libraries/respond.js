const {encrypt} = require('../libraries/encrypt.js');
const {authSecureEncrypt} = require('./authSecure.js');

module.exports = {
  async respond({req,res,status=200,body=''}={}){
    const {key,secret} = req;

    return res.status(status).send(authSecureEncrypt(secret,await encrypt(key,body)));
  }
};
