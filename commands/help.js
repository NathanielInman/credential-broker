import { helpHeader } from './helpHeader.js'
import { helpCommands } from './helpCommands.js'
import { helpStart } from './helpStart.js'
import { helpInit } from './helpInit.js'
import { helpUserAdd } from './helpUserAdd.js'
import { helpUserDelete } from './helpUserDelete.js'
import { helpUserModify } from './helpUserModify.js'
import { helpUserGet } from './helpUserGet.js'
import { helpScopeAdd } from './helpScopeAdd.js'
import { helpScopeDelete } from './helpScopeDelete.js'
import { helpScopeModify } from './helpScopeModify.js'
import { helpGet } from './helpGet.js'
import { helpAdd } from './helpAdd.js'
import { helpDelete } from './helpDelete.js'
import { helpModify } from './helpModify.js'
import { helpAbandonment } from './helpAbandonment.js'
import { helpWipe } from './helpWipe.js'

export function help(command = '') {
  if (command === 'start') {
    helpStart()
  } else if (command === 'init') {
    helpInit()
  } else if (command === 'wipe') {
    helpWipe()
  } else if (command.includes('user-add')) {
    helpUserAdd()
  } else if (command.includes('user-delete')) {
    helpUserDelete()
  } else if (command.includes('user-modify')) {
    helpUserModify()
  } else if (command.includes('user-get')) {
    helpUserGet()
  } else if (command.includes('scope-add')) {
    helpScopeAdd()
  } else if (command.includes('scope-delete')) {
    helpScopeDelete()
  } else if (command.includes('scope-modify')) {
    helpScopeModify()
  } else if (command.includes('get')) {
    helpGet()
  } else if (command.includes('add')) {
    helpAdd()
  } else if (command.includes('delete')) {
    helpDelete()
  } else if (command.includes('modify')) {
    helpModify()
  } else if (command.includes('abandonment')) {
    helpAbandonment()
  } else if (['info', 'stats', 'version'].includes(command)) {
    helpHeader()
  } else {
    helpHeader()
    helpCommands()
  } // end if
}
