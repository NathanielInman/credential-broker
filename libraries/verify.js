import * as openpgp from 'openpgp'

export async function verify(key, text) {
  const publicKey = await openpgp.readKey({ armoredKey: key })
  const message = await openpgp.readCleartextMessage({ cleartextMessage: text })

  const verificationResult = await openpgp.verify({
    message,
    verificationKeys: publicKey
  })

  const { verified } = verificationResult.signatures[0]
  try {
    await verified
    return JSON.parse(verificationResult.data)
  } catch {
    return false
  }
}
