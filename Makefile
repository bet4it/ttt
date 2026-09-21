# Makefile for ttt - terminal text editor

.PHONY: all test build run clean fmt lint chaos chaos-docker chaos-docker-build profiler

all: build

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# Auto-detect if fff_c is installed via pkg-config
ifeq ($(shell pkg-config --exists fff_c 2>/dev/null && echo 1),1)
    TAGS ?= -tags fff
    export CGO_CFLAGS ?= $(shell pkg-config --cflags fff_c)
    ifeq ($(STATIC),1)
        UNAME_S := $(shell uname -s 2>/dev/null)
        ifeq ($(UNAME_S),Darwin)
            FFF_LIBDIR := $(shell pkg-config --variable=libdir fff_c)
            export CGO_LDFLAGS ?= $(FFF_LIBDIR)/libfff_c.a $(shell pkg-config --libs --static fff_c | sed 's/-lfff_c//')
        else ifneq (,$(findstring MINGW,$(UNAME_S)))
            WIN_LIBS := $(shell pkg-config --libs --static fff_c | sed -E 's/-lfff_c//; s/-lwindows\.[0-9.]+//g; s/-lwinapi_[a-zA-Z0-9_]+//g')
            export CGO_LDFLAGS ?= -Wl,-Bstatic $(shell pkg-config --libs fff_c) -Wl,-Bdynamic $(WIN_LIBS) -lws2_32 -luserenv -lntdll -lbcrypt -ladvapi32 -lole32 -lrpcrt4 -lcrypt32 -lsecur32
        else ifneq (,$(findstring MSYS,$(UNAME_S)))
            WIN_LIBS := $(shell pkg-config --libs --static fff_c | sed -E 's/-lfff_c//; s/-lwindows\.[0-9.]+//g; s/-lwinapi_[a-zA-Z0-9_]+//g')
            export CGO_LDFLAGS ?= -Wl,-Bstatic $(shell pkg-config --libs fff_c) -Wl,-Bdynamic $(WIN_LIBS) -lws2_32 -luserenv -lntdll -lbcrypt -ladvapi32 -lole32 -lrpcrt4 -lcrypt32 -lsecur32
        else
            export CGO_LDFLAGS ?= -Wl,-Bstatic $(shell pkg-config --libs fff_c) -Wl,-Bdynamic $(shell pkg-config --libs --static fff_c | sed 's/-lfff_c//') -lz -lpthread -ldl -lm
        endif
    else
        export CGO_LDFLAGS ?= $(shell pkg-config --libs fff_c)
    endif
endif

build:
	go build $(TAGS) -ldflags="-s -w -X main.version=$(VERSION)" -o bin/ttt ./cmd/ttt

test:
	go test $(TAGS) ./...

run: build
	./bin/ttt

fmt:
	gofmt -w .

lint:
	golangci-lint run

vet:
	go vet ./...

chaos: chaos-docker-build
	mkdir -p chaos-output
	docker run --rm -v $(PWD)/chaos-output:/output --entrypoint /chaos-test ttt-chaos \
		-test.run TestChaosMonkey -test.v -test.timeout 15m

chaos-docker-build:
	docker build -t ttt-chaos -f tests/chaos/Dockerfile .

chaos-docker:
	mkdir -p chaos-output
	docker run --rm -v $(PWD)/chaos-output:/output ttt-chaos

# Usage: CHAOS_REPLAY=chaos-output/crash-<seed>-<iter>.json make chaos-replay
chaos-replay: chaos-docker-build
	docker run --rm -v $(PWD)/chaos-output:/output \
		-e CHAOS_REPLAY=/output/$(notdir $(CHAOS_REPLAY)) \
		--entrypoint /chaos-test ttt-chaos -test.run TestChaosReplay -test.v

profiler:
	go build -tags profiler -ldflags="-X main.version=$(VERSION)" -o bin/ttt-profiler ./cmd/ttt

clean:
	rm -rf bin/
	find . -name '*.test' -delete
