const {encrypt} = require('./authPGP.js');
const {authSecureEncrypt} = require('./authSecure.js');

module.exports = {
  async respond({req,res,status=200,body=''}={}){
    const {key,secret} = req,
          pgpEncrypted = await encrypt(key,JSON.stringify(body)),
          sessionEncrypted = authSecureEncrypt(secret,pgpEncrypted);

    return res.status(status).send(sessionEncrypted);
  }
};
