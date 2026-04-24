BINARY  := bedrock-limiter
CMD     := ./cmd/bedrock-limiter
LDFLAGS := -ldflags="-s -w"

.PHONY: build clean

build:
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-amd64        $(CMD)
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-linux-arm64        $(CMD)
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-amd64       $(CMD)
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY)-darwin-arm64       $(CMD)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY)-windows-amd64.exe  $(CMD)

clean:
	rm -f bin/$(BINARY)-*
