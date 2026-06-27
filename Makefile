.PHONY: build test vet clean

BINARY := gitops-vault

build:
	go build -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
	rm -rf .vault/

run-encrypt: build
	./$(BINARY) encrypt $(ARGS)

run-decrypt: build
	./$(BINARY) decrypt $(ARGS)

run-scan: build
	./$(BINARY) scan $(ARGS)
