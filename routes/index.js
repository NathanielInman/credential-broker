const express = require('express');
const initialize = require('./initialize.js');
const userAdd = require('./userAdd.js');
const router = express.Router({mergeParams: true});

router.use('/initialize', initialize.router);
router.use('/userAdd', userAdd.router);

module.exports = {router};
