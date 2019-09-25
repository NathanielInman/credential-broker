const openpgp = require('openpgp');

module.exports = {
  async verify(key,text){
    const publicKeys = (await openpgp.key.readArmored(key)).keys,
          {signatures,data} = await openpgp.verify({
            message: await openpgp.cleartext.readArmored(text),
            publicKeys
          });

    return signatures.every(s=>s.valid)?JSON.parse(data):false;
  }
};
