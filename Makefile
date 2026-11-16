.PHONY: test test-v demo bench race vet stress imports fix

test:
	go test ./...

test-v:
	go test ./... -v

demo:
	go run example/main.go

bench:
	go test ./tests -run '^$$' -bench 'BenchmarkEngine|BenchmarkStopLookup|BenchmarkDepthSnapshot|BenchmarkConcurrentBook|BenchmarkFOKWithOCO|BenchmarkDenseLevelCancel|BenchmarkFOKUnrelatedOCO' -benchmem -count=3

race:
	go test -race ./... -count=1

vet:
	go vet ./...

stress:
	go run ./cmd/loadtest -rate 1000 -duration 30s -workers 8

imports:
	goimports -w .

fix:
	gofmt -s -w .
