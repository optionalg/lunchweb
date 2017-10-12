install:
	go install

run: install
	lunchweb

# Requires reflex: go get github.com/cespare/reflex
watch:
	reflex -s -r "\.go$$" make run
