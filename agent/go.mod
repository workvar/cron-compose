module github.com/croncompose/croncompose/agent

go 1.25

replace github.com/croncompose/croncompose/proto => ../proto

require (
	github.com/creack/pty v1.1.21
	github.com/croncompose/croncompose/proto v0.0.0-00010101000000-000000000000
	github.com/oklog/ulid/v2 v2.1.0
	github.com/robfig/cron/v3 v3.0.1
	go.etcd.io/bbolt v1.3.11
	google.golang.org/grpc v1.66.0
	google.golang.org/protobuf v1.34.2
)

require (
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240604185151-ef581f913117 // indirect
)
