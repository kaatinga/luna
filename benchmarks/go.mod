module github.com/kaatinga/luna/benchmarks

go 1.26

require (
	github.com/jellydator/ttlcache/v3 v3.4.0
	github.com/kaatinga/luna v0.0.0
)

require golang.org/x/sync v0.15.0 // indirect

replace github.com/kaatinga/luna => ../
