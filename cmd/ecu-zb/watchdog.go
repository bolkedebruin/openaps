package main

// watchdogEnabled reports whether the runtime radio watchdog should run. It
// needs the inv-driver bus (the pairing adapter that backs its Probe/Recover)
// and a known operating PAN to re-arm to; with neither there is nothing it can
// drive or restore, so it stays off.
func watchdogEnabled(haveBus bool, opPAN uint16) bool {
	return haveBus && opPAN != 0
}
