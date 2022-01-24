module github.com/IBM-Cloud/go-etcd-rules

go 1.15

replace (
	github.com/coreos/bbolt => go.etcd.io/bbolt v1.3.6
	google.golang.org/grpc => google.golang.org/grpc v1.43.0
)

require (
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/prometheus/client_golang v1.11.0
	github.com/stretchr/testify v1.7.0
	go.etcd.io/etcd/api/v3 v3.5.1 // indirect
	go.etcd.io/etcd/client/v3 v3.5.1 // indirect
	go.uber.org/tools v0.0.0-20190618225709-2cfd321de3ee // indirect
	go.uber.org/zap v1.17.0
	golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4
	honnef.co/go/tools v0.0.1-2019.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)
