.PHONY: all build test vet demo verify verify-gui clean

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

verify: build test vet
	mkdir -p artifacts
	./bin/the-line render --preset esses --view 2d --time 5 --out artifacts/esses-2d.png
	./bin/the-line render --preset banked --view 3d --time 5 --out artifacts/banked-3d.png
	./bin/the-line render --preset rally --view 3d --time 8 --out artifacts/rally-3d.png
	./bin/the-line animate --preset banked --view 3d --out artifacts/banked-animation.gif

verify-gui: build
	mkdir -p artifacts
	./bin/the-line-studio --preset banked --demo --frames 1800 --file artifacts/edited.json --capture artifacts/editor.png --report artifacts/editor-report.json

clean:
	rm -rf bin
