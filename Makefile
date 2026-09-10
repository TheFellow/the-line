.PHONY: all build test vet demo verify clean

all: test build

build:
	go build -o bin/the-line ./main/cli
	go build -o bin/the-line-studio ./main/gui

test:
	go test ./...

vet:
	go vet ./...

demo:
	go run ./main/gui

verify:
	mkdir -p artifacts
	go test ./...
	go vet ./...
	go run ./main/cli render --preset esses --view 2d --out artifacts/esses-2d.png
	go run ./main/cli render --preset banked --view 3d --out artifacts/banked-3d.png

clean:
	rm -rf bin
