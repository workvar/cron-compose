//go:build windows

package main

// Windows has no IANA zoneinfo database, so time.LoadLocation("Asia/Kolkata")
// would fail and every job with a timezone would refuse to schedule. Embedding
// Go's copy (about 450 KB) makes job timezones behave exactly as on Linux.
import _ "time/tzdata"
