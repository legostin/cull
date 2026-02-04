package main

// eagerBirthTime controls whether extractBirthTime is called during quickScanDir.
// On Linux, birth time requires an extra statx() syscall per file, so we defer it
// until the user actually sorts by creation time.
const eagerBirthTime = false
