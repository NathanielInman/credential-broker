const {helpHeader} = require('./helpHeader.js');
const {helpCommands} = require('./helpCommands.js');
const {helpStart} = require('./helpStart.js');
const {helpInit} = require('./helpInit.js');
const {helpUserAdd} = require('./helpUserAdd.js');
const {helpUserDelete} = require('./helpUserDelete.js');
const {helpUserModify} = require('./helpUserModify.js');
const {helpUserGet} = require('./helpUserGet.js');
const {helpScopeAdd} = require('./helpScopeAdd.js');
const {helpScopeDelete} = require('./helpScopeDelete.js');
const {helpScopeModify} = require('./helpScopeModify.js');
const {helpGet} = require('./helpGet.js');
const {helpAdd} = require('./helpAdd.js');
const {helpDelete} = require('./helpDelete.js');
const {helpModify} = require('./helpModify.js');
const {helpAbandonment} = require('./helpAbandonment.js');
const {helpWipe} = require('./helpWipe.js');

module.exports = {
  help(command=''){
    if(command==='start'){
      helpStart();
    }else if(command==='init'){
      helpInit();
    }else if(command==='wipe'){
      helpWipe();
    }else if(command.includes('user-add')){
      helpUserAdd();
    }else if(command.includes('user-delete')){
      helpUserDelete();
    }else if(command.includes('user-modify')){
      helpUserModify();
    }else if(command.includes('user-get')){
      helpUserGet();
    }else if(command.includes('scope-add')){
      helpScopeAdd();
    }else if(command.includes('scope-delete')){
      helpScopeDelete();
    }else if(command.includes('scope-modify')){
      helpScopeModify();
    }else if(command.includes('get')){
      helpGet();
    }else if(command.includes('add')){
      helpAdd();
    }else if(command.includes('delete')){
      helpDelete();
    }else if(command.includes('modify')){
      helpModify();
    }else if(command.includes('abandonment')){
      helpAbandonment();
    }else if(['info','stats','version'].includes(command)){
      helpHeader();
    }else{
      helpHeader();
      helpCommands();
    } //end if
  }
};
