package main

// mountPointsUnderFn resolves mount points below a scan root; a variable so
// tests can inject fake mount tables.
var mountPointsUnderFn = mountPointsUnder
