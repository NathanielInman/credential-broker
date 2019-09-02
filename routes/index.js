const express = require('express');
const initialize = require('./initialize.js');
const userAdd = require('./userAdd.js');
const userGet = require('./userGet.js');
const userGetAll = require('./userGetAll.js');
const router = express.Router({mergeParams: true});

router.use('/initialize', initialize.router);
router.use('/userAdd', userAdd.router);
router.use('/userGet', userGet.router);
router.use('/userGetAll', userGetAll.router);

module.exports = {router};
