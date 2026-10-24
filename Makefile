export GOCACHE := $(CURDIR)/.cache/go-build

.PHONY: run build test vet android android-run touch

run:
	go run .

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -o bin/ancientartofwar .

test:
	go test ./...

vet:
	go vet ./...

android:
	./scripts/build-android.sh

android-run:
	./scripts/run-android.sh

touch:
	go run . -touch
