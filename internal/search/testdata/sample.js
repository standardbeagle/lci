// Sample JavaScript file for testing

// Exported function
export function exportedFunction(param) {
  console.log(param);
  return param.toUpperCase();
}

// Non-exported function
function privateFunction() {
  return 'private';
}

// Exported class
export class ExportedClass {
  constructor(name) {
    this.name = name;
    this._privateProp = null;
  }

  publicMethod() {
    return this.name;
  }

  _privateMethod() {
    return this._privateProp;
  }
}

// Non-exported class
class PrivateClass {
  constructor() {
    this.value = 0;
  }
}

// Exported variable
export const EXPORTED_CONST = 'public';

// Non-exported variable
const PRIVATE_CONST = 'private';

let globalState = null;
