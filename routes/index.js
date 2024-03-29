const express = require('express');
const authSecure = require('./authSecure.js');
const authIdentify = require('./authIdentify.js');
const authChallenge = require('./authChallenge.js');
const auth2FAInitialize = require('./auth2FAInitialize.js');
const auth2FAValidate = require('./auth2FAValidate.js');
const userInitialize = require('./userInitialize.js');
const userAdd = require('./userAdd.js');
const userGet = require('./userGet.js');
const userGetAll = require('./userGetAll.js');
const userModify = require('./userModify.js');
const userDelete = require('./userDelete.js');
const scopeAdd = require('./scopeAdd.js');
const scopeGet = require('./scopeGet.js');
const scopeGetAll = require('./scopeGetAll.js');
const scopeModify = require('./scopeModify.js');
const scopeDelete = require('./scopeDelete.js');
const secretAdd = require('./secretAdd.js');
const secretModify = require('./secretModify.js');
const secretDelete = require('./secretDelete.js');
const router = express.Router({mergeParams: true});

// Secure communications & authentication routes
router.use('/authSecure', authSecure.router);
router.use('/authIdentify', authIdentify.router);
router.use('/authChallenge', authChallenge.router);
router.use('/auth2FAInitialize', auth2FAInitialize.router);
router.use('/auth2FAValidate', auth2FAValidate.router);

// General routes
router.use('/userInitialize', userInitialize.router);
router.use('/userAdd', userAdd.router);
router.use('/userGet', userGet.router);
router.use('/userGetAll', userGetAll.router);
router.use('/userModify', userModify.router);
router.use('/userDelete', userDelete.router);
router.use('/scopeAdd', scopeAdd.router);
router.use('/scopeGet', scopeGet.router);
router.use('/scopeGetAll', scopeGetAll.router);
router.use('/scopeModify', scopeModify.router);
router.use('/scopeDelete', scopeDelete.router);
router.use('/secretAdd', secretAdd.router);
router.use('/secretModify', secretModify.router);
router.use('/secretDelete', secretDelete.router);

module.exports = {router};
