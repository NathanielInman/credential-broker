const express = require('express');
const initialize = require('./initialize.js');
const userAdd = require('./userAdd.js');
const userGet = require('./userGet.js');
const userGetAll = require('./userGetAll.js');
const userModify = require('./userModify.js');
const userDelete = require('./userDelete.js');
const scopeAdd = require('./scopeAdd.js');
const router = express.Router({mergeParams: true});

router.use('/initialize', initialize.router);
router.use('/userAdd', userAdd.router);
router.use('/userGet', userGet.router);
router.use('/userGetAll', userGetAll.router);
router.use('/userModify', userModify.router);
router.use('/userDelete', userDelete.router);
router.use('/scopeAdd', scopeAdd.router);

module.exports = {router};
