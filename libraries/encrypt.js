const openpgp = require('openpgp');
const fs = require('fs');

module.exports = {
  async encrypt(key,text){
    const publicKey = fs.readFileSync('./id_rsa.pub','utf8');

    const publicKeys = (await openpgp.key.readArmored(publicKey)).keys,
          {data} = await openpgp.encrypt({
            message: openpgp.message.fromText(text),
            publicKeys
          });

    return data;
  }
};
