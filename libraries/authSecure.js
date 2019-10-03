const crypto = require('crypto');
const fetch = require('node-fetch');

module.exports = {
  async authSecure(user){
    const diffieHellman = crypto.createDiffieHellman(512),
          prime = diffieHellman.getPrime('base64'),
          client = crypto.createDiffieHellman(prime,'base64'),
          key = client.generateKeys('base64'),
          id = crypto.createHash('md5').update(user.name).digest('hex'),
          serverKey = await fetch(`${user.remoteIP}/authSecure`,{
            method: 'POST',
            headers: {id,key,prime}
          }).then(res=> res.json()),
          secret = client.computeSecret(serverKey,'base64','base64');

    return secret;
  }
};
