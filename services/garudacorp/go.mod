module github.com/garudapass/gpass/services/garudacorp

go 1.25.0
toolchain go1.25.8

require (
	github.com/garudapass/gpass/packages/golib v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.12.3
)

replace github.com/garudapass/gpass/packages/golib => ../../packages/golib
