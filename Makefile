LEVEL?="INFO"

.PHONY: all test clean build api ui serve test
all: clean test deps build serve

clean:
	rm -rf ./go-plaid
build:
	go build -a -ldflags 'main.buildTime=$(date)' .
deps:
	npm i
api:
	./go-plaid --logging $(LEVEL) --port "8080"
ui:
	npm run build && npm start
serve:
	make -j api ui
test:
	go test ./... -test.v
