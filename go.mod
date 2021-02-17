module github.com/IBM-Cloud/go-etcd-rules

go 1.12

require (
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/coreos/etcd v3.3.24+incompatible
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/mattn/goveralls v0.0.7 // indirect
	github.com/prometheus/client_golang v0.9.3
	github.com/sirupsen/logrus v1.4.1 // indirect
	github.com/spf13/cobra v1.1.1 // indirect
	github.com/stretchr/testify v1.3.0
	go.etcd.io/etcd v3.3.25+incompatible // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550 // indirect
	golang.org/x/net v0.0.0-20200226121028-0de0cce0169b
	sigs.k8s.io/yaml v1.2.0 // indirect
)

// Due to a bad import in a dependency, golint doesn't play well with 'go get -u ./...':
// https://github.com/golang/lint/issues/436 & https://github.com/golang/go/issues/30833
replace github.com/golang/lint => golang.org/x/lint v0.0.0-20190313153728-d0100b6bd8b3
