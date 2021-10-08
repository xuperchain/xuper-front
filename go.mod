module github.com/xuperchain/xuper-front

go 1.13

replace github.com/tjfoc/gmsm v1.2.3 => github.com/bd4gm/gmsm v1.2.6

require (
	github.com/dgraph-io/badger/v3 v3.2103.1 // indirect
	github.com/gammazero/deque v0.1.0 // indirect
	github.com/go-sql-driver/mysql v1.5.0
	github.com/golang/protobuf v1.4.3
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/jmoiron/sqlx v1.2.1-0.20190826204134-d7d95172beb5
	github.com/mattn/go-sqlite3 v2.0.3-0.20200109094304-d51eaf3b3471+incompatible
	github.com/pelletier/go-toml v1.4.0 // indirect
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.6.2
	github.com/xuperchain/crypto v0.0.0-20201028025054-4d560674bcd6
	github.com/xuperchain/log15 v0.0.0-20190620081506-bc88a9198230
	github.com/xuperchain/xuperchain v0.0.0-20210927115948-7a094acb608e
	github.com/xuperchain/xupercore v0.0.0-20210927035201-1ce8d8deeec2
	golang.org/x/net v0.0.0-20201021035429-f5854403a974
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013
	google.golang.org/grpc v1.35.0
	google.golang.org/protobuf v1.26.0-rc.1
)
