module github.com/bolke/ecu-sunspec

go 1.26.3

require (
	github.com/bolke/inv-driver v0.0.0-00010101000000-000000000000
	github.com/simonvetter/modbus v1.6.3
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

require (
	github.com/goburrow/serial v0.1.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/bolke/inv-driver => ../inv-driver
