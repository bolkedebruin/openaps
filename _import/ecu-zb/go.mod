module github.com/bolke/ecu-zb

go 1.26.3

require (
	github.com/bolke/inv-driver v0.0.0-00010101000000-000000000000
	golang.org/x/sys v0.42.0
)

require google.golang.org/protobuf v1.36.11 // indirect

replace github.com/bolke/inv-driver => ../inv-driver
