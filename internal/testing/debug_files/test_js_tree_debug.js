// Test various anonymous function forms in JavaScript

// Arrow function
const arrow = () => {
    console.log("inside arrow function");
};

// Traditional anonymous function
const traditional = function() {
    console.log("inside traditional anonymous");
};

// Anonymous function in callback
setTimeout(function() {
    console.log("inside timeout callback");
}, 1000);

// Arrow function in map
[1, 2, 3].map(x => x * 2);

// Nested anonymous functions
const outer = () => {
    const inner = function() {
        console.log("inside nested anonymous");
    };
    inner();
};