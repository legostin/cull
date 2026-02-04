package main

// eagerBirthTime controls whether extractBirthTime is called during quickScanDir.
// On macOS, birth time comes from the Stat_t already returned by Info(), so it's free.
const eagerBirthTime = true
