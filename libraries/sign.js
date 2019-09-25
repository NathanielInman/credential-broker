const openpgp = require('openpgp');
const fs = require('fs');

module.exports = {
  async sign(user,text){
    const privateKey = fs.readFileSync('./id_rsa').toString(),
          privateKeys = (await openpgp.key.readArmored(privateKey)).keys;

    await Promise.all(privateKeys.map(k=> k.decrypt(user.email)));
    const {data} = await openpgp.sign({
            message: openpgp.cleartext.fromText(text),
            privateKeys
          });

    return data;
  }
};
