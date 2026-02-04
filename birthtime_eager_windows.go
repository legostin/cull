package main

// eagerBirthTime controls whether extractBirthTime is called during quickScanDir.
// On Windows, birth time comes from Win32FileAttributeData already returned by Info(), so it's free.
const eagerBirthTime = true
