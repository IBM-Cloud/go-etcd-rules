module github.com/IBM-Cloud/go-etcd-rules

go 1.12

require (
	cloud.google.com/go v0.37.2 // indirect
	github.com/GoogleCloudPlatform/cloudsql-proxy v0.0.0-20190329203910-c904b696df3c // indirect
	github.com/Shopify/sarama v1.21.0 // indirect
	github.com/aclements/go-gg v0.0.0-20170323211221-abd1f791f5ee // indirect
	github.com/aclements/go-moremath v0.0.0-20180329182055-b1aff36309c7 // indirect
	github.com/coreos/bbolt v1.3.2 // indirect
	github.com/coreos/etcd v3.3.12+incompatible
	github.com/coreos/go-semver v0.2.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/gliderlabs/ssh v0.1.3 // indirect
	github.com/go-logfmt/logfmt v0.4.0 // indirect
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef // indirect
	github.com/golang/protobuf v1.3.1 // indirect
	github.com/golang/snappy v0.0.1 // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/google/pprof v0.0.0-20190309163659-77426154d546 // indirect
	github.com/googleapis/gax-go v2.0.2+incompatible // indirect
	github.com/gorilla/mux v1.7.0 // indirect
	github.com/gorilla/websocket v1.4.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20190212212710-3befbb6ad0cc // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.8.5 // indirect
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/jonboulle/clockwork v0.1.0 // indirect
	github.com/kisielk/errcheck v1.2.0 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/kr/pty v1.1.4 // indirect
	github.com/mattn/go-sqlite3 v1.10.0 // indirect
	github.com/mattn/goveralls v0.0.2 // indirect
	github.com/onsi/ginkgo v1.8.0 // indirect
	github.com/onsi/gomega v1.5.0 // indirect
	github.com/pkg/errors v0.8.1 // indirect
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90 // indirect
	github.com/prometheus/procfs v0.0.0-20190328153300-af7bedc223fb // indirect
	github.com/rogpeppe/fastuuid v1.0.0 // indirect
	github.com/sirupsen/logrus v1.4.0 // indirect
	github.com/soheilhy/cmux v0.1.4 // indirect
	github.com/stretchr/testify v1.3.0
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	github.com/ugorji/go v1.1.1 // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	go.etcd.io/bbolt v1.3.2 // indirect
	go.uber.org/atomic v1.3.2 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.9.1
	go4.org v0.0.0-20190313082347-94abd6928b1d // indirect
	golang.org/x/build v0.0.0-20190328203648-c72a0eda0790 // indirect
	golang.org/x/crypto v0.0.0-20190325154230-a5d413f7728c // indirect
	golang.org/x/exp v0.0.0-20190321205749-f0864edee7f3 // indirect
	golang.org/x/image v0.0.0-20190321063152-3fc05d484e9f // indirect
	golang.org/x/lint v0.0.0-20190313153728-d0100b6bd8b3 // indirect
	golang.org/x/mobile v0.0.0-20190327163128-167ebed0ec6d // indirect
	golang.org/x/net v0.0.0-20190328230028-74de082e2cca
	golang.org/x/oauth2 v0.0.0-20190319182350-c85d3e98c914 // indirect
	golang.org/x/perf v0.0.0-20190312170614-0655857e383f // indirect
	golang.org/x/sys v0.0.0-20190329044733-9eb1bfa1ce65 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	golang.org/x/tools v0.0.0-20190401162208-5d16bdd7b52e // indirect
	google.golang.org/appengine v1.5.0 // indirect
	google.golang.org/genproto v0.0.0-20190327125643-d831d65fe17d // indirect
	google.golang.org/grpc v1.19.1 // indirect
	gopkg.in/yaml.v2 v2.2.2 // indirect
	honnef.co/go/tools v0.0.0-20190319011948-d116c56a00f3 // indirect
)

// Due to a bad import in a dependency, golint doesn't play well with 'go get -u ./...':
// https://github.com/golang/lint/issues/436 & https://github.com/golang/go/issues/30833
replace github.com/golang/lint => golang.org/x/lint v0.0.0-20190313153728-d0100b6bd8b3
