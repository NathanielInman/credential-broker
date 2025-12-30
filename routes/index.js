import express from 'express'
import authSecure from './authSecure.js'
import authIdentify from './authIdentify.js'
import authChallenge from './authChallenge.js'
import auth2FAInitialize from './auth2FAInitialize.js'
import auth2FAValidate from './auth2FAValidate.js'
import userInitialize from './userInitialize.js'
import userAdd from './userAdd.js'
import userGet from './userGet.js'
import userGetAll from './userGetAll.js'
import userModify from './userModify.js'
import userDelete from './userDelete.js'
import scopeAdd from './scopeAdd.js'
import scopeGet from './scopeGet.js'
import scopeGetAll from './scopeGetAll.js'
import scopeModify from './scopeModify.js'
import scopeDelete from './scopeDelete.js'
import secretAdd from './secretAdd.js'
import secretModify from './secretModify.js'
import secretDelete from './secretDelete.js'
import wipe from './wipe.js'
import wipeCancel from './wipeCancel.js'

const router = express.Router({ mergeParams: true })

// Secure communications & authentication routes
router.use('/authSecure', authSecure.router)
router.use('/authIdentify', authIdentify.router)
router.use('/authChallenge', authChallenge.router)
router.use('/auth2FAInitialize', auth2FAInitialize.router)
router.use('/auth2FAValidate', auth2FAValidate.router)

// General routes
router.use('/userInitialize', userInitialize.router)
router.use('/userAdd', userAdd.router)
router.use('/userGet', userGet.router)
router.use('/userGetAll', userGetAll.router)
router.use('/userModify', userModify.router)
router.use('/userDelete', userDelete.router)
router.use('/scopeAdd', scopeAdd.router)
router.use('/scopeGet', scopeGet.router)
router.use('/scopeGetAll', scopeGetAll.router)
router.use('/scopeModify', scopeModify.router)
router.use('/scopeDelete', scopeDelete.router)
router.use('/secretAdd', secretAdd.router)
router.use('/secretModify', secretModify.router)
router.use('/secretDelete', secretDelete.router)
router.use('/wipe', wipe.router)
router.use('/wipeCancel', wipeCancel.router)

export default { router }
