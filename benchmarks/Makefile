# Reproduce the numbers in the root README.
#
#   make bench     # full suite, 6 runs, latency + allocs (writes new.txt)
#   make memory    # heap retained after fill+delete
#   make stat      # compare new.txt against a committed baseline with benchstat
#
# benchstat: go install golang.org/x/perf/cmd/benchstat@latest

COUNT ?= 6

.PHONY: bench memory stat

bench:
	go test -run '^$$' -bench . -benchmem -count $(COUNT) | tee new.txt

memory:
	go test -run TestMemoryFootprint -v

stat:
	benchstat baseline.txt new.txt
