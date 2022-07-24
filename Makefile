LEVEL?="INFO"
PLAID_SECRET?="SECRET"
PLAID_CLIENT_ID?="CLIENT_ID"

.PHONY: all test clean build api ui serve test
all: clean test deps build

clean:
	rm -rf ./go-plaid
build:
	go build -a -ldflags 'main.buildTime=$(date)' .
deps:
	npm i
api:
	PLAID_SECRET=$(PLAID_SECRET) PLAID_CLIENT_ID=$(PLAID_CLIENT_ID) ./go-plaid --logging $(LEVEL) --port "8080"
ui:
	npm run build && npm start
serve:
	make -j api ui
test:
	go test ./... -test.v
