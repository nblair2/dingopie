BIN=dingopie.bin

build:
	@go build -o $(BIN) .
	@sudo setcap 'cap_net_admin=+ep' ./dingopie.bin

clean:
	@rm -rf $(BIN)